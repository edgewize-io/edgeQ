package metrics

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/pkg/constants"
	"github.com/golang/snappy"
	clientmodel "github.com/prometheus/client_model/go"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"reflect"
	"sort"
	"time"
)

const (
	sumStr          = "_sum"
	countStr        = "_count"
	bucketStr       = "_bucket"
	successExitCode = 0
	failureExitCode = 1
)

var MetricMetadataTypeValue = map[string]int32{
	"UNKNOWN":        0,
	"COUNTER":        1,
	"GAUGE":          2,
	"HISTOGRAM":      3,
	"GAUGEHISTOGRAM": 4,
	"SUMMARY":        5,
	"INFO":           6,
	"STATESET":       7,
}

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

func NewMetricCollector(cfg config.PromMetrics) *MetricCollector {
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
	close(mc.StopCh)
}

func (mc *MetricCollector) scrape() (err error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", mc.MetricAddr))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	wreq, err := MetricTextToWriteRequest(resp.Body, map[string]string{})
	if err != nil {
		return
	}

	err = sendRemoteWriteRequest(context.Background(), GetEdgeProxyAddress(), wreq)
	return
}

func MetricTextToWriteRequest(input io.Reader, labels map[string]string) (*prompb.WriteRequest, error) {
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(input)
	if err != nil {
		return nil, err
	}
	return MetricFamiliesToWriteRequest(mf, labels)
}

func MetricFamiliesToWriteRequest(mf map[string]*dto.MetricFamily, extraLabels map[string]string) (*prompb.WriteRequest, error) {
	wr := &prompb.WriteRequest{}

	// build metric list
	sortedMetricNames := make([]string, 0, len(mf))
	for metric := range mf {
		sortedMetricNames = append(sortedMetricNames, metric)
	}
	// sort metrics name in lexicographical order
	sort.Strings(sortedMetricNames)

	for _, metricName := range sortedMetricNames {
		// Set metadata writerequest
		mtype := MetricMetadataTypeValue[mf[metricName].Type.String()]
		metadata := prompb.MetricMetadata{
			MetricFamilyName: mf[metricName].GetName(),
			Type:             prompb.MetricMetadata_MetricType(mtype),
			Help:             mf[metricName].GetHelp(),
		}
		wr.Metadata = append(wr.Metadata, metadata)

		for _, metric := range mf[metricName].Metric {
			labels := makeLabelsMap(metric, metricName, extraLabels)
			if err := makeTimeseries(wr, labels, metric); err != nil {
				return wr, err
			}
		}
	}
	return wr, nil
}

func makeTimeseries(wr *prompb.WriteRequest, labels map[string]string, m *dto.Metric) error {
	var err error

	timestamp := m.GetTimestampMs()
	if timestamp == 0 {
		timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	}

	switch {
	case m.Gauge != nil:
		toTimeseries(wr, labels, timestamp, m.GetGauge().GetValue())
	case m.Counter != nil:
		toTimeseries(wr, labels, timestamp, m.GetCounter().GetValue())
	case m.Summary != nil:
		metricName := labels[model.MetricNameLabel]
		// Preserve metric name order with first quantile labels timeseries then sum suffix timeseries and finally count suffix timeseries
		// Add Summary quantile timeseries
		quantileLabels := make(map[string]string, len(labels)+1)
		for key, value := range labels {
			quantileLabels[key] = value
		}

		for _, q := range m.GetSummary().Quantile {
			quantileLabels[model.QuantileLabel] = fmt.Sprint(q.GetQuantile())
			toTimeseries(wr, quantileLabels, timestamp, q.GetValue())
		}
		// Overwrite label model.MetricNameLabel for count and sum metrics
		// Add Summary sum timeseries
		labels[model.MetricNameLabel] = metricName + sumStr
		toTimeseries(wr, labels, timestamp, m.GetSummary().GetSampleSum())
		// Add Summary count timeseries
		labels[model.MetricNameLabel] = metricName + countStr
		toTimeseries(wr, labels, timestamp, float64(m.GetSummary().GetSampleCount()))

	case m.Histogram != nil:
		metricName := labels[model.MetricNameLabel]
		// Preserve metric name order with first bucket suffix timeseries then sum suffix timeseries and finally count suffix timeseries
		// Add Histogram bucket timeseries
		bucketLabels := make(map[string]string, len(labels)+1)
		for key, value := range labels {
			bucketLabels[key] = value
		}
		for _, b := range m.GetHistogram().Bucket {
			bucketLabels[model.MetricNameLabel] = metricName + bucketStr
			bucketLabels[model.BucketLabel] = fmt.Sprint(b.GetUpperBound())
			toTimeseries(wr, bucketLabels, timestamp, float64(b.GetCumulativeCount()))
		}
		// Overwrite label model.MetricNameLabel for count and sum metrics
		// Add Histogram sum timeseries
		labels[model.MetricNameLabel] = metricName + sumStr
		toTimeseries(wr, labels, timestamp, m.GetHistogram().GetSampleSum())
		// Add Histogram count timeseries
		labels[model.MetricNameLabel] = metricName + countStr
		toTimeseries(wr, labels, timestamp, float64(m.GetHistogram().GetSampleCount()))

	case m.Untyped != nil:
		toTimeseries(wr, labels, timestamp, m.GetUntyped().GetValue())
	default:
		err = errors.New("unsupported metric type")
	}
	return err
}

func toTimeseries(wr *prompb.WriteRequest, labels map[string]string, timestamp int64, value float64) {
	var ts prompb.TimeSeries
	ts.Labels = makeLabels(labels)
	ts.Samples = []prompb.Sample{
		{
			Timestamp: timestamp,
			Value:     value,
		},
	}
	wr.Timeseries = append(wr.Timeseries, ts)
}

func makeLabels(labelsMap map[string]string) []prompb.Label {
	// build labels name list
	sortedLabelNames := make([]string, 0, len(labelsMap))
	for label := range labelsMap {
		sortedLabelNames = append(sortedLabelNames, label)
	}
	// sort labels name in lexicographical order
	sort.Strings(sortedLabelNames)

	var labels []prompb.Label
	for _, label := range sortedLabelNames {
		labels = append(labels, prompb.Label{
			Name:  label,
			Value: labelsMap[label],
		})
	}
	return labels
}

func makeLabelsMap(m *dto.Metric, metricName string, extraLabels map[string]string) map[string]string {
	// build labels map
	labels := make(map[string]string, len(m.Label)+len(extraLabels))
	labels[model.MetricNameLabel] = metricName

	// add extra labels
	for key, value := range extraLabels {
		labels[key] = value
	}

	// add metric labels
	for _, label := range m.Label {
		labelname := label.GetName()
		if labelname == model.JobLabel {
			labelname = fmt.Sprintf("%s%s", model.ExportedLabelPrefix, labelname)
		}

		labelValue := label.GetValue()
		if labelValue == "" || labelname == "" {
			continue
		}

		labels[labelname] = label.GetValue()
	}

	return labels
}

func GetEdgeProxyAddress() string {
	nodeIP := os.Getenv("NODE_IP")
	proxyPort := os.Getenv("METRICS_PROXY_PORT")
	if proxyPort == "" {
		proxyPort = constants.DefaultMetricsProxyPort
	}

	return fmt.Sprintf("http://%s:%s/api/v1/write", nodeIP, proxyPort)
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
	data, err := req.Marshal()
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
