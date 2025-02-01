package proxy

import (
	"context"
	"fmt"
	v1 "github.com/edgewize/edgeQ/api/modelfulx/v1alpha"
	proto "github.com/edgewize/edgeQ/mindspore_serving/proto"
	"github.com/edgewize/edgeQ/pkg/utils"
	"k8s.io/klog/v2"
)

func (s *Server) Predict(ctx context.Context, in *proto.PredictRequest) (*proto.PredictReply, error) {
	klog.V(2).Infof("--- Predict ---")

	requestID := utils.ReqUUID()
	request := &v1.PredictRequest{
		Id:        requestID,
		Mindspore: in,
	}

	if err := s.dispatch.queue.Push(request); err != nil {
		return nil, err
	}

	// wait for response, dispatch will call back when response is ready.
	holderResponse := s.dispatch.Wait(ctx, requestID)
	klog.V(2).Infof("response: %v", holderResponse)
	response, ok := holderResponse.Data.(*proto.PredictReply)
	if !ok || response == nil {
		klog.Errorf("parse predict reply failed")
		return nil, fmt.Errorf("parse predict reply failed")
	}

	return response, nil
}
