package otel

//import (
//	"go.opentelemetry.io/otel/exporters/prometheus"
//	mc "go.opentelemetry.io/otel/metric"
//	"go.opentelemetry.io/otel/sdk/metric"
//	"log"
//)
//
//type(
//	Counter =
//)
//
//var (
//	Meter mc.Meter
//)
//
//func init() {
//	// The exporter embeds a default OpenTelemetry Reader and
//	// implements prometheus.Collector, allowing it to be used as
//	// both a Reader and Collector.
//	exporter, err := prometheus.New()
//	if err != nil {
//		log.Fatal(err)
//	}
//	provider := metric.NewMeterProvider(metric.WithReader(exporter))
//	Meter = provider.Meter("github.com/open-telemetry/opentelemetry-go/example/prometheus")
//
//}
