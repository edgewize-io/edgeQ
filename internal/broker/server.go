package broker

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	be "github.com/edgewize/edgeQ/internal/broker/endpoint"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	proto "github.com/edgewize/edgeQ/mindspore_serving/proto"
	"github.com/edgewize/edgeQ/pkg/endpoint"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	Config          *config.Config
	listener        net.Listener
	Endpoint        endpoint.QosEndpoint
	ServingClient   proto.MSServiceClient
	lock            sync.RWMutex
	PromMetricsSrv  *metrics.PromMetricsServer
	MetricCollector *metrics.MetricCollector
	PProfSrv        *metrics.PProfServer
}

func (s *Server) PrepareRun() (err error) {
	s.Endpoint, err = be.GetBrokerEndpoint(s.Config)
	if err != nil {
		return
	}

	s.PromMetricsSrv = metrics.NewPromMetricsSrv(s.Config.PromMetrics)

	s.MetricCollector = metrics.NewMetricCollector(s.Config.PromMetrics)

	s.PProfSrv = metrics.NewPProfSrv(s.Config.BaseOptions.ProfEnable)

	if s.Endpoint == nil {
		return fmt.Errorf("broker server is nil")
	}

	return nil
}

func (s *Server) StopComponents() {
	s.Endpoint.Stop()
	s.PromMetricsSrv.Stop()
	s.MetricCollector.Stop()
	s.PProfSrv.Stop()
}

func (s *Server) Run(ctx context.Context) (err error) {
	klog.V(0).Infof("broker components about to start")
	klog.V(0).Infof("Start listening on %s", s.Config.Broker.Addr)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	go func() {
		select {
		case <-ctx.Done():
		}

		s.StopComponents()
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
		return s.Endpoint.Start(ctx)
	})

	eg.Go(func() error {
		klog.V(0).Infof("prometheus metric server start listening on %s", s.Config.PromMetrics.Addr)
		return s.PromMetricsSrv.Run()
	})

	eg.Go(func() error {
		return s.PProfSrv.Run()
	})

	eg.Go(func() error {
		return s.MetricCollector.Run(ctx)
	})

	err = eg.Wait()
	time.Sleep(5 * time.Second)
	klog.V(0).Infof("all broker components stopped..")
	return
}
