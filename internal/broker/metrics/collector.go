package metrics

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	clientmodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/prometheus/prompb"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"reflect"
	"sort"
	"time"
)

type MetricPoint struct {
	Metric  string            `json:"metric"`
	TagsMap map[string]string `json:"tags"`
	Time    int64             `json:"time"`
	Value   float64           `json:"value"`
}

type MetricCollector struct {
	MetricAddr string
	Interval   time.Duration
	StopCh     chan struct{}
}

type sortableLabels []prompb.Label

func (sl *sortableLabels) Len() int           { return len(*sl) }
func (sl *sortableLabels) Swap(i, j int)      { (*sl)[i], (*sl)[j] = (*sl)[j], (*sl)[i] }
func (sl *sortableLabels) Less(i, j int) bool { return (*sl)[i].Name < (*sl)[j].Name }

func NewMetricCollector(cfg *config.PromMetrics) *MetricCollector {
	return &MetricCollector{
		MetricAddr: cfg.Addr,
		Interval:   cfg.ScrapeInterval,
		StopCh:     make(chan struct{}),
	}
}

func (mc *MetricCollector) Run(ctx context.Context) (err error) {
	klog.V(3).Info("metric collector started..")

Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		case <-mc.StopCh:
			break Loop
		case <-time.After(mc.Interval):
			err = mc.scrape()
			if err != nil {
				klog.Error("scrape metrics failed, err: %v", err)
			}
		}
	}

	klog.V(3).Info("metric collector stopped..")
	return
}

func (mc *MetricCollector) Stop() {
	mc.StopCh <- struct{}{}
}

func (mc *MetricCollector) scrape() (err error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", mc.MetricAddr))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	timeSeries, err := parsePrometheusResponse(resp.Body)
	if err != nil {
		return
	}

	if len(timeSeries) == 0 {
		klog.Errorf("no time series to forward to receive endpoint")
		return
	}

	wreq := &prompb.WriteRequest{Timeseries: timeSeries}

	err = sendRemoteWriteRequest(context.Background(), GetEdgeProxyAddress(), wreq)
	return
}

func GetEdgeProxyAddress() string {
	nodeIP := os.Getenv("NODE_IP")
	proxyPort := os.Getenv("METRICS_PROXY_PORT")
	if proxyPort == "" {
		proxyPort = constants.DefaultMetricsProxyPort
	}

	return fmt.Sprintf("https://%s:%s/push", nodeIP, proxyPort)
}

func parsePrometheusResponse(reader io.Reader) ([]prompb.TimeSeries, error) {
	var timeSeries []prompb.TimeSeries
	var parser expfmt.TextParser
	metricFamily, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	for _, f := range metricFamily {
		if f == nil {
			klog.Warningf("metricFamily is nil")
			continue
		}

		switch *f.Type {
		case clientmodel.MetricType_COUNTER, clientmodel.MetricType_GAUGE, clientmodel.MetricType_UNTYPED:
		default:
			klog.V(4).Infof("metric type %s not supported", f.Type.String())
			continue
		}

		for _, m := range f.Metric {
			if reflect.ValueOf(m).IsNil() {
				klog.Warningf("metric is nil")
				continue
			}

			var ts prompb.TimeSeries

			labelPairs := []prompb.Label{{
				Name:  "__name__",
				Value: *f.Name,
			}}

			dedup := make(map[string]struct{})
			for _, l := range m.Label {
				// Skip empty labels.
				if *l.Name == "" || *l.Value == "" {
					continue
				}
				// Check for duplicates
				if _, ok := dedup[*l.Name]; ok {
					continue
				}
				labelPairs = append(labelPairs, prompb.Label{
					Name:  *l.Name,
					Value: *l.Value,
				})
				dedup[*l.Name] = struct{}{}
			}

			s := prompb.Sample{
				Timestamp: timestamp,
			}

			switch *f.Type {
			case clientmodel.MetricType_COUNTER:
				s.Value = *m.Counter.Value
			case clientmodel.MetricType_GAUGE:
				s.Value = *m.Gauge.Value
			case clientmodel.MetricType_UNTYPED:
				s.Value = *m.Untyped.Value
			default:
				return nil, fmt.Errorf("metric type %s not supported", f.Type.String())
			}

			ts.Labels = append(ts.Labels, labelPairs...)
			sortLabels(ts.Labels)

			ts.Samples = append(ts.Samples, s)

			timeSeries = append(timeSeries, ts)
		}
	}

	return timeSeries, nil

}

func sortLabels(labels []prompb.Label) {
	lset := sortableLabels(labels)
	sort.Sort(&lset)
}

func sendRemoteWriteRequest(ctx context.Context, url string, req *prompb.WriteRequest) error {
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	compressed := snappy.Encode(nil, data)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(compressed))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send remote write request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
