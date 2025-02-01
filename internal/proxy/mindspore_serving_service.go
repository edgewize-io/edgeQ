package proxy

import (
	"context"
	pb "github.com/edgewize/modelmesh/mindspore_serving/proto"
	"k8s.io/klog/v2"
)

type MSServiceService struct {
	client pb.MSServiceClient
}

func NewMSServiceService(client pb.MSServiceClient) *MSServiceService {
	return &MSServiceService{client: client}
}

func (s *MSServiceService) Predict(ctx context.Context, req *pb.PredictRequest) (*pb.PredictReply, error) {
	//ret, nil := s.client.Predict(ctx, req)
	//klog.V(0).Infof("Predict request: %v, %v", req, ret)
	klog.V(0).Infof("Predict request: %v", req)
	return &pb.PredictReply{}, nil
}
