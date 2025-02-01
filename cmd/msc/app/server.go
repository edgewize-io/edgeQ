package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/edgewize/edgeQ/cmd/msc/app/options"
	"github.com/edgewize/edgeQ/internal/msc"
	"github.com/edgewize/edgeQ/internal/msc/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"os/signal"
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
	dnsNames := []string{
		s.Config.WebhookServiceName,
		s.Config.WebhookServiceName + "." + s.Config.Namespace,
		s.Config.WebhookServiceName + "." + s.Config.Namespace + "." + "svc",
	}

	commonName := s.Config.WebhookServiceName + "." + s.Config.Namespace + "." + "svc"

	org := "edgewize"
	caPEM, certPEM, certKeyPEM, err := msc.GenerateCert([]string{org}, dnsNames, commonName)
	if err != nil {
		klog.Errorf("Failed to generate ca and certificate key pair: %v", err)
		return
	}

	pair, err := tls.X509KeyPair(certPEM.Bytes(), certKeyPEM.Bytes())
	if err != nil {
		klog.Errorf("Failed to load certificate key pair: %v", err)
		return
	}

	klog.Infof("Initializing the kube client...")
	kubeconfig := os.Getenv("KUBECONFIG")
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return err
	}

	err = msc.CreateOrUpdateMutatingWebhookConfiguration(clientset, caPEM, s.Config.WebhookServiceName, s.Config.Namespace, int32(s.Config.WebhookPort))
	if err != nil {
		klog.Errorf("Failed to create or update the mutating webhook configuration: %v", err)
		return
	}

	webhookServer := &msc.WebhookServer{
		Server: &http.Server{
			Addr:      fmt.Sprintf("0.0.0.0:%v", s.Config.WebhookPort),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
		ProxySidecar:     s.ProxySidecar,
		BrokerSidecar:    s.BrokerSidecar,
		Clientset:        clientset,
		CurrentNamespace: s.Config.Namespace,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(msc.WebhookApplicationInjectPath, webhookServer.InjectApplication)
	mux.HandleFunc(msc.WebHookModelInjectPath, webhookServer.InjectModel)
	readyFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}

	mux.HandleFunc("/healthz", readyFunc)
	mux.HandleFunc("/readyz", readyFunc)
	webhookServer.Server.Handler = mux

	go func() {
		if err := webhookServer.Server.ListenAndServeTLS("", ""); err != nil {
			klog.Fatalf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	_ = webhookServer.Server.Shutdown(context.Background())
	return
}
