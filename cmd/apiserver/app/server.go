package app

import (
	"context"
	"github.com/edgewize/edgeQ/cmd/apiserver/app/options"
	apiserverconfig "github.com/edgewize/edgeQ/pkg/apiserver/config"
	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sync"
)

func NewAPIServerCommand() *cobra.Command {
	s := options.NewServerRunOptions()

	// Load configuration from file
	conf, err := apiserverconfig.TryLoadFromDisk()
	if err == nil {
		s = &options.ServerRunOptions{
			GenericServerRunOptions: s.GenericServerRunOptions,
			APIServerConfig:         conf,
			SchemeOnce:              sync.Once{},
		}
	} else {
		klog.Fatal("Failed to load configuration from disk", err)
	}

	cmd := &cobra.Command{
		Use: "edgeQ-apiserver",
		Long: `The EdgeQ API server validates and configures data for the API objects. 
The API Server services REST operations and provides the frontend to the
cluster's shared state through which all other components interact.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := s.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			return Run(s, apiserverconfig.WatchConfigChange(), signals.SetupSignalHandler())
		},
		SilenceUsage: true,
	}

	return cmd
}

func Run(s *options.ServerRunOptions, configCh <-chan apiserverconfig.APIServerConfig, ctx context.Context) error {
	ictx, cancelFunc := context.WithCancel(context.TODO())
	errCh := make(chan error)
	defer close(errCh)
	go func() {
		if err := run(s, ictx); err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case <-ctx.Done():
			cancelFunc()
			return nil
		case cfg := <-configCh:
			cancelFunc()
			s.APIServerConfig = &cfg
			ictx, cancelFunc = context.WithCancel(context.TODO())
			go func() {
				if err := run(s, ictx); err != nil {
					errCh <- err
				}
			}()
		case err := <-errCh:
			cancelFunc()
			return err
		}
	}
}

func run(s *options.ServerRunOptions, ctx context.Context) error {
	apiserver, err := s.NewAPIServer(ctx.Done())
	if err != nil {
		return err
	}

	err = apiserver.PrepareRun(ctx.Done())
	if err != nil {
		return err
	}

	err = apiserver.Run(ctx)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
