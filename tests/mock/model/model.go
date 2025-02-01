package main

import (
	"context"
	"fmt"
	brokerconfig "github.com/edgewize/edgeQ/internal/broker/config"
	mindspore_serving_proto "github.com/edgewize/edgeQ/mindspore_serving/proto"
	"github.com/edgewize/edgeQ/pkg/transport/grpc"
	"github.com/edgewize/edgeQ/pkg/utils"
	"k8s.io/klog/v2"
	"net"
)

var _handler = &handler{}

type handler struct {
}

func (h *handler) Predict(ctx context.Context, in *mindspore_serving_proto.PredictRequest) (*mindspore_serving_proto.PredictReply, error) {
	klog.V(0).Info("Predict")
	return &mindspore_serving_proto.PredictReply{
		ServableSpec: &mindspore_serving_proto.ServableSpec{
			Name: "Predict test",
		},
	}, nil
}

func main() {
	stopCh := make(chan struct{})
	go func() {
		err := RunMockModelService()
		klog.Errorf("mock model server down, %v", err)
		stopCh <- struct{}{}
	}()

	<-stopCh
}

func RunMockModelService() error {
	cfg := brokerconfig.New()
	s := grpc.NewServer(cfg.BrokerServer)

	mindspore_serving_proto.RegisterMSServiceServer(s, _handler)

	addr := ":5500" // mock serving server address
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
