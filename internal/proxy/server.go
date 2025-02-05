package proxy

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/internal/proxy/config"
	he "github.com/edgewize/edgeQ/internal/proxy/endpoint/http"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
)

type Server struct {
	Config   *config.Config
	Endpoint *he.HttpProxyServer //Broker grpc server
}

func (s *Server) PrepareRun() (err error) {
	s.Endpoint, err = he.NewHttpProxyServer(s.Config)
	if err != nil {
		return
	}

	if s.Endpoint == nil {
		return fmt.Errorf("proxy server is nil")
	}

	return nil
}

func (s *Server) Run(ctx context.Context) (err error) {
	klog.V(0).Infof("proxy components start")
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	klog.V(0).Infof("Start listening on %s", s.Config.Proxy.Addr)

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	go func() {
		select {
		case <-ctx.Done():
			s.Endpoint.Stop()
		}
	}()

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

	eg.Go(func() error {
		return s.Endpoint.Start()
	})

	err = eg.Wait()
	klog.V(0).Infof("proxy components stopped..")
	return
}
