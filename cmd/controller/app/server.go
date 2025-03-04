package app

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/cmd/controller/app/options"
	"github.com/edgewize/edgeQ/internal/controller"
	"github.com/edgewize/edgeQ/internal/controller/config"
	appsv1alpha1 "github.com/edgewize/edgeQ/pkg/apis/apps/v1alpha1"
	"github.com/edgewize/edgeQ/pkg/simple/client/k8s"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"syscall"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func ModelServiceManagerCmd() (cmd *cobra.Command) {
	s := options.NewServerRunOptions()

	conf, err := config.TryLoadFromDisk()
	if err != nil {
		klog.Fatal("Failed to load configuration from disk", err)
	}

	s = &options.ServerRunOptions{
		Config: conf,
	}
	ret, _ := yaml.Marshal(conf)
	fmt.Println(string(ret))

	cmd = &cobra.Command{
		Use:  "Model Service Controller",
		Long: `The Model Service Controller makes it possible for model mesh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := s.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			return Run(s)
		},
		SilenceUsage: true,
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, 0)
	})

	return
}

func Run(s *options.ServerRunOptions) (err error) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return startWebhookServer(ctx, s)
	})

	eg.Go(func() error {
		select {
		case sig := <-signalChan:
			klog.Infof("Received signal \"%v\", shutting down...", sig)
			cancel()
			return fmt.Errorf("received signal %s", sig)
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err = eg.Wait()
	if err != nil {
		klog.Errorf("Exiting with error: %v", err)
	}

	klog.Infof("controller shutting down...")
	return
}

func startControllerManager(ctx context.Context, s *options.ServerRunOptions) error {
	kubernetesClient, err := k8s.NewKubernetesClient(s.KubernetesOptions)
	if err != nil {
		klog.Errorf("Failed to create kubernetes clientset %v", err)
		return err
	}

	mgrOptions := manager.Options{
		HealthProbeBindAddress: s.HealthProbeBindAddress,
		Metrics: metricsserver.Options{
			BindAddress: s.MetricsBindAddress,
		},
	}

	if s.LeaderElect {
		mgrOptions.LeaderElection = s.LeaderElect
		mgrOptions.LeaderElectionID = "edge-qos-controller-manager-leader-election"
		mgrOptions.LeaseDuration = &s.LeaderElection.LeaseDuration
		mgrOptions.RetryPeriod = &s.LeaderElection.RetryPeriod
		mgrOptions.RenewDeadline = &s.LeaderElection.RenewDeadline
	}

	mgr, err := manager.New(kubernetesClient.Config(), mgrOptions)
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %v", err)
	}

	_ = appsv1alpha1.AddToScheme(mgr.GetScheme())

	//metav1.AddToGroupVersion(mgr.GetScheme(), metav1.SchemeGroupVersion)

}

func addControllers(ctx context.Context, mgr manager.Manager) error {

}

func startWebhookServer(ctx context.Context, s *options.ServerRunOptions) (err error) {
	ws, err := controller.NewWebHookServer(
		s.Config.WebhookServiceName,
		s.Config.Namespace,
		int32(s.Config.WebhookPort),
		s.ProxySidecar,
		s.BrokerSidecar,
	)

	if err != nil {
		return
	}

	go func(ctx context.Context) {
		<-ctx.Done()
		_ = ws.Stop()
		klog.Infof("Webhook server shutting down")
	}(ctx)

	err = ws.Serve()
	return
}
