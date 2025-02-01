package broker

import (
	"context"
	"fmt"
	v1 "github.com/edgewize/modelmesh/api/modelfulx/v1alpha"
	"github.com/edgewize/modelmesh/internal/broker/config"
	"github.com/edgewize/modelmesh/internal/broker/metrics"
	proto "github.com/edgewize/modelmesh/mindspore_serving/proto"
	xgrpc "github.com/edgewize/modelmesh/pkg/transport/grpc"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type Dispatch struct {
	client   proto.MSServiceClient
	queue    *Queue
	poolSize int
}

func NewDispatch(cfg *config.Dispatch) (*Dispatch, error) {
	client := xgrpc.NewServingClient(cfg.Client)
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	queue, err := NewQueue(cfg.Queue)
	return &Dispatch{
		poolSize: cfg.PoolSize,
		client:   client,
		queue:    queue,
	}, err
}

// Run dispatch request to serving server
// @TODO call serving server
// 1. pull Request from queues
// 2. call serving server
// 3. call stream send response
func (d *Dispatch) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i <= d.poolSize; i++ {
		wg.Add(1)
		go func() {
			d.startWorker(ctx, &wg)
		}()
	}
	wg.Wait()
	return nil
}

func (d *Dispatch) startWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wrapReq := d.queue.BlockingPop(ctx)
		if wrapReq == nil {
			continue
		}
		req := wrapReq.RawRequest
		metricLabelValues := []string{
			metrics.POD_NAME,
			metrics.CONTAINER_NAME,
			wrapReq.Method,
			wrapReq.ServiceGroup,
		}

		brokerProcessMillisecond := float64((time.Now().UnixNano() - wrapReq.ReceivedUnixNanoTime) / int64(time.Millisecond))
		metrics.BrokerProcessingTimeTotal.WithLabelValues(metricLabelValues...).Add(brokerProcessMillisecond)
		resp, err := d.client.Predict(ctx, req.Request.Mindspore)
		if err != nil {
			klog.Errorf("Predict error: %v", err)
			continue
		}

		klog.V(2).Infof("Send Callback %v,%v", resp, err)
		// testing WRR
		if metrics.RequestSleepSecond > 0 {
			time.Sleep(time.Duration(metrics.RequestSleepSecond) * time.Second)
		}

		metrics.BrokerSendRequestTotal.WithLabelValues(metricLabelValues...).Inc()
		backendProcessMillisecond := float64((time.Now().UnixNano() - wrapReq.ReceivedUnixNanoTime) / int64(time.Millisecond))
		metrics.BackendProcessingTimeTotal.WithLabelValues(metricLabelValues...).Add(backendProcessMillisecond)
		err = req.Send(&v1.PredictReply{Id: req.Request.Id, Mindspore: resp})
		if err != nil {
			klog.Errorf("Callback error: %v", err)
			continue
		}

		metrics.BrokerSendResponseTotal.WithLabelValues(metricLabelValues...).Inc()
	}
}
