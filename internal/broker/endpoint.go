package broker

import (
	"context"
	v1 "github.com/edgewize/edgeQ/api/modelfulx/v1alpha"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	"github.com/edgewize/edgeQ/pkg/utils"
	"io"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

const (
	timestampFormat = time.StampNano
)

type WrapPredictRequest struct {
	ReceivedUnixNanoTime int64
	RawRequest           *PredictRequest
	Method               string
	ServiceGroup         string
}

type PredictRequest struct {
	Request v1.PredictRequest
	Send    func(*v1.PredictReply) error
}

func (s *Server) Predict(stream v1.MFService_PredictServer) error {
	klog.V(0).Infof("--- Broker Receive Predict ---")
	serviceGroup := utils.ServiceGroupFromContext(stream.Context())

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			klog.Errorf("receive error in Predict, %v", err)
			continue
		}

		metrics.BrokerReceivedRequestTotal.WithLabelValues(
			metrics.POD_NAME,
			metrics.CONTAINER_NAME,
			s.Config.Schedule.Method,
			serviceGroup,
		).Inc()
		//应用按照配置推入不同的  group
		rawRequest := &PredictRequest{
			Request: *in,
			Send:    stream.Send,
		}
		s.Push(ctx, serviceGroup, &WrapPredictRequest{
			RawRequest:           rawRequest,
			ReceivedUnixNanoTime: time.Now().UnixNano(),
			Method:               s.Config.Schedule.Method,
			ServiceGroup:         serviceGroup,
		})

		if !s.disableFlowControl() {
			s.RequestNotifyChan <- struct{}{}
		}
	}
}

func (s *Server) disableFlowControl() bool {
	return s.Config.Schedule.DisableFlowControl
}

func (s *Server) Process(ctx context.Context) error {
	if s.disableFlowControl() {
		return s.ProcessWithoutFlowControl(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.RequestNotifyChan:
		}

		s.flowControlHandler(ctx)
	}
}

func (s *Server) ProcessWithoutFlowControl(ctx context.Context) error {
	// aggregate chan receives requests from all service group queues
	aggChan := make(chan *WrapPredictRequest)
	var wg sync.WaitGroup
	for _, svgQueue := range s.queue.queues {
		wg.Add(1)
		go func(q chan *WrapPredictRequest) {
			for _req := range q {
				select {
				case aggChan <- _req:
				case <-ctx.Done():
					wg.Done()
					return
				}
			}
		}(svgQueue.ch)
	}

	go func(aggCh chan *WrapPredictRequest) {
		for _req := range aggCh {
			metrics.DispatchRequestTotal.WithLabelValues(
				metrics.POD_NAME,
				metrics.CONTAINER_NAME,
				s.Config.Schedule.Method,
				_req.ServiceGroup,
			).Inc()

			s.dispatch.queue.Push(ctx, _req)
		}
	}(aggChan)

	wg.Wait()
	close(aggChan)
	klog.V(0).Infof("processWithoutFlowControl aggregate chan stopped..")
	return nil
}

// TODO optimizing
func (s *Server) flowControlHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ret, err := s.scheduler.Pick(picker.PickInfo{})
		if err != nil {
			klog.Errorf("scheduler pick error: %v", err)
			continue
		}
		group := ret.Resource.ResourceName()
		if s.transGroupRequest(ctx, group) {
			return
		}
	}
}

func (s *Server) Push(ctx context.Context, group string, in *WrapPredictRequest) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	queue, err := s.queue.Select(group)
	if err != nil {
		klog.Errorf("scheduler pick error: %v", err)
		return err
	}

	queue.Push(ctx, in)
	return nil
}

func (s *Server) transGroupRequest(ctx context.Context, group string) (done bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	queue, err := s.queue.Select(group)
	if err != nil {
		klog.Errorf("scheduler pick error: %v", err)
		return
	}

	metricLabelValues := []string{
		metrics.POD_NAME,
		metrics.CONTAINER_NAME,
		s.Config.Schedule.Method,
		group,
	}

	request := queue.Pop(ctx)
	if request == nil {
		metrics.PickNilRequestFromQueueTotal.WithLabelValues(metricLabelValues...).Inc()
		return
	}

	s.dispatch.queue.Push(ctx, request)

	metrics.DispatchRequestTotal.WithLabelValues(metricLabelValues...).Inc()
	transReqProcessMillisecond := float64((time.Now().UnixNano() - request.ReceivedUnixNanoTime) / int64(time.Millisecond))
	metrics.TransRequestProcessingTimeTotal.WithLabelValues(metricLabelValues...).Add(transReqProcessMillisecond)
	done = true
	return
}
