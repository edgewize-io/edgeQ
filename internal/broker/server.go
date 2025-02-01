package broker

import (
	"context"
	"fmt"
	v1 "github.com/edgewize/edgeQ/api/modelfulx/v1alpha"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	proto "github.com/edgewize/edgeQ/mindspore_serving/proto"
	xgrpc "github.com/edgewize/edgeQ/pkg/transport/grpc"
	"github.com/edgewize/edgeQ/pkg/utils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"net"
	"sync"
	"time"
)

type Server struct {
	Config            *config.Config
	listener          net.Listener
	Endpoint          *grpc.Server //Broker grpc server
	ServingClient     proto.MSServiceClient
	queue             *QueuePool
	dispatch          *Dispatch
	scheduler         *Scheduler
	done              <-chan struct{}
	lock              sync.RWMutex
	PromMetricsSrv    *metrics.PromMetricsServer
	MetricCollector   *metrics.MetricCollector
	PprofSrv          *metrics.PprofServer
	RequestNotifyChan chan struct{}
}

func (s *Server) PrepareRun(done <-chan struct{}) (err error) {
	s.done = done
	s.Endpoint = xgrpc.NewServer(s.Config.BrokerServer)

	s.queue, err = NewQueuePool(s.Config.Queue, s.Config.ServiceGroups)
	if err != nil {
		return fmt.Errorf("queues is nil")
	}

	s.RequestNotifyChan = NewRequestNotifyChan(s.Config.Queue, s.Config.ServiceGroups)

	s.scheduler, err = NewScheduler(s.Config.Schedule)
	if err != nil {
		return err
	}
	s.scheduler.RegeneratePicker(s.Config.ServiceGroups)

	s.dispatch, err = NewDispatch(s.Config.Dispatch)
	if err != nil {
		return err
	}

	s.PromMetricsSrv = metrics.NewPromMetricsSrv(s.Config.PromMetrics)

	s.MetricCollector = metrics.NewMetricCollector(s.Config.PromMetrics)

	s.PprofSrv = metrics.NewPprofSrv(s.Config.BaseOptions.ProfEnable)

	if s.Endpoint == nil {
		return fmt.Errorf("broker server is nil")
	}
	v1.RegisterMFServiceServer(s.Endpoint, s)

	return nil
}

func (s *Server) StopComponents() {
	s.Endpoint.GracefulStop()
	s.PromMetricsSrv.Stop()
	s.MetricCollector.Stop()
	s.PprofSrv.Stop()
}

func (s *Server) Run(ctx context.Context) (err error) {
	klog.V(0).Infof("broker components about to start")
	go func() {
		select {
		case <-ctx.Done():
		case <-s.done:
		}

		s.StopComponents()
	}()

	addr := s.Config.BrokerServer.Addr
	klog.V(0).Infof("Start listening on %s", addr)

	network, address := utils.ExtractNetAddress(addr)
	if network != "tcp" {
		return fmt.Errorf("unsupported protocol %s: %v", network, addr)
	}

	if s.listener, err = net.Listen(network, address); err != nil {
		return fmt.Errorf("unable to listen: %w", err)
	}

	var eg errgroup.Group
	eg.Go(func() error {
		return s.dispatch.Run(ctx)
	})
	eg.Go(func() error {
		return s.Process(ctx)
	})
	eg.Go(func() error {
		return s.Endpoint.Serve(s.listener)
	})

	eg.Go(func() error {
		klog.V(0).Infof("prometheus metric server start listening on %s", s.Config.PromMetrics.Addr)
		return s.PromMetricsSrv.Run()
	})

	eg.Go(func() error {
		return s.PprofSrv.Run()
	})

	eg.Go(func() error {
		return s.MetricCollector.Run(ctx)
	})

	err = eg.Wait()
	time.Sleep(5 * time.Second)
	klog.V(0).Infof("all broker components stopped..")
	return
}
