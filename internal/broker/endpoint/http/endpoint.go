package http

import (
	"context"
	"fmt"
	brokerCfg "github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	sch "github.com/edgewize/edgeQ/internal/broker/scheduler"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/edgewize/edgeQ/pkg/queue"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"
)

type HttpBrokerServer struct {
	ReverseProxy      *httputil.ReverseProxy
	Broker            *http.Server
	QueuePool         *queue.QueuePool
	Addr              string
	Scheduler         *sch.Scheduler
	ScheduleMethod    string
	WorkPool          chan struct{}
	stopChan          chan struct{}
	EnableFlowControl bool
}

func GetTargetPortFromEnv() (int32, error) {
	targetPortEnv := os.Getenv("TARGET_PORT")
	targetPort, err := strconv.Atoi(targetPortEnv)
	if err != nil {
		klog.Errorf("Failed to convert TARGET_PORT to int: %v", err)
		return 0, err
	}

	return int32(targetPort), err
}

func NewHttpBrokerServer(cfg *brokerCfg.Config) (*HttpBrokerServer, error) {
	targetPort, err := GetTargetPortFromEnv()
	if err != nil {
		return nil, err
	}

	targetURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", targetPort))
	if err != nil {
		return nil, err
	}

	reverseProxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(targetURL)
		},
	}
	reverseProxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Broker.HTTP.DialTimeout, // TCP 连接超时
			KeepAlive: cfg.Broker.HTTP.KeepAlive,   // 保持长连接时间
		}).DialContext,
		ResponseHeaderTimeout: cfg.Broker.HTTP.HeaderTimeout,
		IdleConnTimeout:       cfg.Broker.HTTP.IdleConnTimeout,
		MaxIdleConns:          cfg.Broker.HTTP.MaxIdleConns,
		MaxConnsPerHost:       cfg.Broker.HTTP.MaxConnsPerHost,
	}

	server := &http.Server{
		Addr:              cfg.Broker.Addr,
		ReadTimeout:       cfg.Broker.HTTP.ReadTimeout,
		WriteTimeout:      cfg.Broker.HTTP.WriteTimeout,
		ReadHeaderTimeout: cfg.Broker.HTTP.HeaderTimeout,
		IdleTimeout:       cfg.Broker.HTTP.IdleConnTimeout,
	}

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, request *http.Request, e error) {
		klog.Warningf("request backend server error: %v", e)
		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}

	queuePool := queue.NewQueuePool(cfg.Queue, cfg.ServiceGroups)

	newScheduler, err := sch.NewScheduler(cfg.Schedule)
	if err != nil {
		return nil, err
	}

	newScheduler.RegeneratePicker(cfg.ServiceGroups)

	return &HttpBrokerServer{
		ReverseProxy:      reverseProxy,
		Broker:            server,
		Addr:              cfg.Broker.Addr,
		QueuePool:         queuePool,
		Scheduler:         newScheduler,
		WorkPool:          make(chan struct{}, cfg.Broker.WorkPoolSize),
		stopChan:          make(chan struct{}),
		EnableFlowControl: cfg.Schedule.EnableFlowControl,
		ScheduleMethod:    cfg.Schedule.Method,
	}, nil
}

func (s *HttpBrokerServer) enableFlowControl() bool {
	return s.EnableFlowControl
}

func (s *HttpBrokerServer) Start(ctx context.Context) error {
	if !s.enableFlowControl() {
		s.Broker.Handler = s.ReverseProxy
	} else {
		s.Broker.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("收到请求\n")
			waitChan, err := s.EnqueueRequests(w, r)
			if err != nil {
				fmt.Printf("请求入队报错，%v\n", err)
				return
			}

			<-waitChan
			fmt.Printf("请求执行完毕，返回\n")
		})

		go s.ProcessRequests(ctx)
	}

	return s.Broker.ListenAndServe()
}

func (s *HttpBrokerServer) Stop() {
	close(s.stopChan)
	_ = s.Broker.Shutdown(context.Background())
}

func (s *HttpBrokerServer) EnqueueRequests(w http.ResponseWriter, r *http.Request) (waitChan chan struct{}, err error) {
	serviceGroup := r.Header.Get(constants.ServiceGroupHeader)
	if serviceGroup == "" {
		serviceGroup = constants.DefaultServiceGroup
	}

	waitChan = make(chan struct{})
	reqItem := &queue.HTTPRequestItem{
		ServiceGroup: serviceGroup,
		Req:          r,
		Writer:       w,
		Ctx:          r.Context(),
		CreateTime:   time.Now(),
		WaitChan:     waitChan,
	}

	err = s.QueuePool.Push(serviceGroup, reqItem)
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	metrics.BrokerReceivedRequestTotalRecord(s.ScheduleMethod, serviceGroup)
	return
}

func (s *HttpBrokerServer) ProcessRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		default:
		}

		item, err := s.QueuePool.Pop(s.Scheduler)
		if err != nil {
			continue
		}

		if item == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		httpItem, ok := item.(*queue.HTTPRequestItem)
		if !ok {
			continue
		}

		metrics.BrokerProcessingDurationRecord(s.ScheduleMethod, httpItem.ResourceName(), time.Since(httpItem.GetCreateTime()).Seconds())

		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case s.WorkPool <- struct{}{}:
			s.HandleRequest(httpItem)
		}
	}
}

func (s *HttpBrokerServer) HandleRequest(item *queue.HTTPRequestItem) {
	defer func() { <-s.WorkPool }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := item.Req.Clone(ctx)
	s.ReverseProxy.ServeHTTP(item.Writer, req)

	metrics.BackendHandledRequestTotalRecord(s.ScheduleMethod, item.ServiceGroup)
	metrics.BackendReceivedRequestTotalRecord(s.ScheduleMethod, item.ServiceGroup, time.Since(item.CreateTime).Seconds())
	close(item.WaitChan)
}
