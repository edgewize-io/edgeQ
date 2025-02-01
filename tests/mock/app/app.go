package main

import (
	"context"
	mindspore_serving_proto "github.com/edgewize/modelmesh/mindspore_serving/proto"
	xconfig "github.com/edgewize/modelmesh/pkg/config"
	"github.com/edgewize/modelmesh/pkg/transport/grpc"
	"k8s.io/klog/v2"
	"time"
)

func main() {
	RunMockApp()
}

func RunMockApp() {
	ctx := context.Background()
	client := grpc.NewServingClient(&xconfig.GRPCClient{
		Addr: ":8080",
	})

	for {
		_, err := client.Predict(ctx, &mindspore_serving_proto.PredictRequest{
			ServableSpec: &mindspore_serving_proto.ServableSpec{
				Name: "Predict test",
			},
		})
		if err != nil {
			klog.Errorf("send request to proxy failed, %v", err)
		}
		klog.V(0).Info("Predict")
		time.Sleep(2 * time.Second)
	}
}
