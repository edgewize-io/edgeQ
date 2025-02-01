package metrics

import (
	"context"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	// DefaultRegisterer and DefaultGatherer are the implementations of the
	// prometheus Registerer and Gatherer interfaces that all metrics operations
	// will use. They are variables so that packages that embed this library can
	// replace them at runtime, instead of having to pass around specific
	// registries.
	DefaultRegisterer = prometheus.DefaultRegisterer
	DefaultGatherer   = prometheus.DefaultGatherer
	UnknownName       = "unknown"
	POD_NAME          string
	CONTAINER_NAME    string
	// for testing
	RequestSleepSecond int = 0
)

type PromMetricsServer struct {
	*http.Server
}

var (
	BrokerReceivedRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_received_request_total",
		Help: "Total number of broker received requests",
	}, []string{"pod", "container", "method", "service_group"})

	BrokerSendRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_send_request_total",
		Help: "Total number of broker received requests",
	}, []string{"pod", "container", "method", "service_group"})

	BrokerSendResponseTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_send_response_total",
		Help: "Total number of broker sending responses",
	}, []string{"pod", "container", "method", "service_group"})

	PickNilRequestFromQueueTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_pick_nil_request_from_queue_total",
		Help: "total number of nil request picked",
	}, []string{"pod", "container", "method", "service_group"})

	DispatchRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_dispatch_request_total",
		Help: "total number of dispatched requests",
	}, []string{"pod", "container", "method", "service_group"})

	BrokerProcessingTimeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_processing_time_total",
		Help: "total processing milliseconds time when the requests are about to be sent to backend",
	}, []string{"pod", "container", "method", "service_group"})

	BackendProcessingTimeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_backend_processing_time_total",
		Help: "total processing milliseconds time when the responses returned from backend",
	}, []string{"pod", "container", "method", "service_group"})

	TransRequestProcessingTimeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_trans_request_processing_time_total",
		Help: "total processing milliseconds time when the requests are pushed to dispatch queue",
	}, []string{"pod", "container", "method", "service_group"})
)

func Register() {
	DefaultRegisterer.MustRegister(BrokerReceivedRequestTotal)
	DefaultRegisterer.MustRegister(BrokerSendRequestTotal)
	DefaultRegisterer.MustRegister(BrokerSendResponseTotal)
	DefaultRegisterer.MustRegister(PickNilRequestFromQueueTotal)
	DefaultRegisterer.MustRegister(DispatchRequestTotal)
	DefaultRegisterer.MustRegister(BrokerProcessingTimeTotal)
	DefaultRegisterer.MustRegister(BackendProcessingTimeTotal)
	DefaultRegisterer.MustRegister(TransRequestProcessingTimeTotal)
}

func (p *PromMetricsServer) Run() error {
	if err := p.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (p *PromMetricsServer) Stop() {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	err := p.Shutdown(shutdownCtx)
	if err != nil {
		klog.Errorf("stop PromMetricsServer failed, [%v]", err)
	}
}

func NewPromMetricsSrv(cfg *config.PromMetrics) *PromMetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(DefaultGatherer, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return &PromMetricsServer{
		Server: server,
	}
}

func init() {
	Register()
	POD_NAME = os.Getenv("POD_NAME")
	if POD_NAME == "" {
		POD_NAME = UnknownName
	}

	CONTAINER_NAME = os.Getenv("CONTAINER_NAME")
	if CONTAINER_NAME == "" {
		CONTAINER_NAME = UnknownName
	}

	sleepSecond, err := strconv.Atoi(os.Getenv("REQUEST_SLEEP_SECOND"))
	if err == nil {
		RequestSleepSecond = sleepSecond
	}
}
