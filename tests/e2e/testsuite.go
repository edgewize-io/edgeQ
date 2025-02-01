package e2e

import (
	"context"
	"fmt"
	proxyoptions "github.com/edgewize/modelmesh/cmd/proxy/app/options"
	brokerconfig "github.com/edgewize/modelmesh/internal/broker/config"
	mindspore_serving_proto "github.com/edgewize/modelmesh/mindspore_serving/proto"
	xconfig "github.com/edgewize/modelmesh/pkg/config"
	"github.com/edgewize/modelmesh/pkg/transport/grpc"
	"github.com/edgewize/modelmesh/pkg/utils"
	"k8s.io/klog/v2"
	"net"
)
import "github.com/go-gota/gota/dataframe"

type RequestInfo struct {
	Name          string
	MethodName    string
	VersionNumber int64
	Count         int
}

type Metric struct {
	DF dataframe.DataFrame
}

func MetricFromStruct(a interface{}) *Metric {
	df := dataframe.LoadStructs(a)
	return &Metric{DF: df}
}

func (m *Metric) Metric() dataframe.DataFrame {
	return m.DF
}

func (m *Metric) GroupBy(fields ...string) map[string]int {
	grouped := m.DF.GroupBy(fields...)
	ret := map[string]int{}
	for i, g := range grouped.GetGroups() {
		ret[i] = g.Nrow()
	}
	return ret
}

func (m *Metric) NRow() int {
	return m.DF.Nrow()
}

type FakeServingServer struct {
	Handler func(ctx context.Context, in *mindspore_serving_proto.PredictRequest) error
}

func (h *FakeServingServer) Predict(ctx context.Context, in *mindspore_serving_proto.PredictRequest) (*mindspore_serving_proto.PredictReply, error) {
	klog.V(0).Info("FakeServingServer.Predict")
	if h.Handler != nil {
		h.Handler(ctx, in)
	}
	return &mindspore_serving_proto.PredictReply{
		ServableSpec: &mindspore_serving_proto.ServableSpec{
			Name: "Predict test",
		},
	}, nil
}

func FakeClent(ctx context.Context, opt *proxyoptions.ServerRunOptions) (mindspore_serving_proto.MSServiceClient, error) {
	addr := opt.ProxyServer.Addr
	client := grpc.NewServingClient(&xconfig.GRPCClient{
		Addr: addr,
	})
	return client, nil
}

func RunFakeServingServer(ctx context.Context, server *FakeServingServer) error {
	cfg, err := brokerconfig.TryLoadFromDisk()
	if err != nil {
		return err
	}
	s := grpc.NewServer(cfg.BrokerServer)

	mindspore_serving_proto.RegisterMSServiceServer(s, server)

	addr := cfg.Dispatch.Client.Addr // mock serving server address
	klog.V(0).Infof("Start listening on %s", addr)

	network, address := utils.ExtractNetAddress(addr)
	if network != "tcp" {
		return fmt.Errorf("unsupported protocol %s: %v", network, addr)
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("unable to listen: %w", err)
	}

	return s.Serve(listener)
}
