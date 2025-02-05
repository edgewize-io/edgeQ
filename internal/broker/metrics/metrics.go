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

	BackendHandledRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_backend_handled_request_total",
		Help: "Total number of backend handled requests",
	}, []string{"pod", "container", "method", "service_group"})

	BrokerProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "broker_processing_duration_seconds",
		Help:    "time taken to process requests in broker queues",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	}, []string{"pod", "container", "method", "service_group"})

	BackendProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "broker_backend_processing_duration_seconds",
		Help:    "time taken to process requests in backend servers",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	}, []string{"pod", "container", "method", "service_group"})
)

func Register() {
	DefaultRegisterer.MustRegister(BrokerReceivedRequestTotal)
	DefaultRegisterer.MustRegister(BackendHandledRequestTotal)
	DefaultRegisterer.MustRegister(BrokerProcessingDuration)
	DefaultRegisterer.MustRegister(BackendProcessingDuration)
}

func BrokerReceivedRequestTotalRecord(method string, serviceGroup string) {
	BrokerReceivedRequestTotal.WithLabelValues(POD_NAME, CONTAINER_NAME, method, serviceGroup).Inc()
}

func BackendHandledRequestTotalRecord(method string, serviceGroup string) {
	BackendHandledRequestTotal.WithLabelValues(POD_NAME, CONTAINER_NAME, method, serviceGroup).Inc()
}

func BrokerProcessingDurationRecord(method string, serviceGroup string, duration float64) {
	BrokerProcessingDuration.WithLabelValues(POD_NAME, CONTAINER_NAME, method, serviceGroup).Observe(duration)
}

func BackendReceivedRequestTotalRecord(method string, serviceGroup string, duration float64) {
	BackendProcessingDuration.WithLabelValues(POD_NAME, CONTAINER_NAME, method, serviceGroup).Observe(duration)
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

func NewPromMetricsSrv(cfg config.PromMetrics) *PromMetricsServer {
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
