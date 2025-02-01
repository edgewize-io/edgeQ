package proxy

import (
	"context"
	"fmt"
	v1 "github.com/edgewize/edgeQ/api/modelfulx/v1alpha"
	"github.com/edgewize/edgeQ/internal/proxy/config"
	"github.com/edgewize/edgeQ/internal/proxy/holder"
	xgrpc "github.com/edgewize/edgeQ/pkg/transport/grpc"
	"github.com/edgewize/edgeQ/pkg/utils"
	"k8s.io/klog/v2"
	"math"
	"time"
)

var (
	defaultMaxRetries = 5
	baseDelay         = 1 * time.Second
)

type Dispatch struct {
	queue        *Queue
	client       v1.MFServiceClient
	holder       holder.Holder
	ServiceGroup string
}

func NewDispatch(cfg *config.Dispatch, group *config.ServiceGroup, holder holder.Holder) (*Dispatch, error) {
	serviceGroup := "default"
	if group != nil && group.Name != "" {
		serviceGroup = group.Name
	}

	client := xgrpc.NewBrokerClient(cfg.Client)
	if client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	queue, err := NewQueue(cfg.Queue, cfg.Timeout)
	return &Dispatch{
		client:       client,
		holder:       holder,
		queue:        queue,
		ServiceGroup: serviceGroup,
	}, err
}

func (d *Dispatch) Run(ctx context.Context) error {
	return d.RunWithReconnect(ctx)
}

func (d *Dispatch) run(ctx context.Context) error {
	klog.V(1).Infof("--- dispatch running ---")
	// Create metadata and context
	ctx = utils.ContextWithServiceGroup(ctx, d.ServiceGroup)
	stream, err := d.client.Predict(ctx)
	if err != nil {
		klog.Errorf("init grpc stream failed, %v", err)
		return err
	}

	defer stream.CloseSend()

	errChan := make(chan error, 1)
	go func() {
		idx := 0
		for {
			in, err := stream.Recv()
			if err != nil {
				klog.Warningf("Failed to receive reply from stream : %v", err)
				errChan <- err
				return
			}

			d.Notify(in.Id, holder.StatusOK, in.Mindspore, "")
			klog.V(1).Infof("Got message [%d]%v:%v", idx, &in, in.Id)
			idx++
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err = <-errChan:
			return err
		default:
		}

		ret := d.queue.Pop(ctx)
		if ret == nil {
			continue
		}

		in := &v1.PredictRequest{
			Id:        ret.Id,
			Mindspore: ret.Mindspore,
		}
		err := stream.Send(in)
		if err != nil {
			klog.Errorf("send stream failed, %v", err)
			d.Notify(in.Id, holder.StatusFailed, nil, err.Error())
		}
		klog.V(2).Infof("Send %v,%v", ret, err)
	}
}

func (d *Dispatch) Notify(id string, status holder.Status, data interface{}, errMessage string) {
	d.holder.Notify(&holder.Response{
		ID:         id,
		Status:     status,
		ErrMessage: errMessage,
		Data:       data,
	})
}

func (d *Dispatch) getNextDelay(num int) time.Duration {
	if num >= defaultMaxRetries {
		num = defaultMaxRetries
	}

	secRetry := math.Pow(2, float64(num))
	klog.Warningf("Retrying connection in %f seconds", secRetry)
	return time.Duration(secRetry) * baseDelay
}

func (d *Dispatch) RunWithReconnect(ctx context.Context) error {
	var lastError error
	retryCount := 0
	for {
		lastError = d.run(ctx)
		select {
		case <-ctx.Done():
			klog.V(0).Infof("dispatch loop cancelled")
			return lastError
		default:
		}

		delay := d.getNextDelay(retryCount)
		time.Sleep(delay)

		retryCount++
	}
}

func (d *Dispatch) Wait(ctx context.Context, id string) holder.Response {
	return d.holder.Wait(ctx, id)
}
