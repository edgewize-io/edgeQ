package broker

import (
	"context"
	"github.com/edgewize/modelmesh/internal/broker/config"
	errs "github.com/edgewize/modelmesh/internal/broker/error"
)

type QueueSelect func() (serviceGroup string, err error)

type QueuePool struct {
	queues map[string]*Queue
}

type Queue struct {
	ch chan *WrapPredictRequest
}

func NewQueuePool(cfg *config.Queue, sc []*config.ServiceGroup) (*QueuePool, error) {
	size := cfg.Size
	qp := &QueuePool{
		queues: map[string]*Queue{},
	}
	for _, group := range sc {
		qp.queues[group.Name] = &Queue{ch: make(chan *WrapPredictRequest, size)}
	}
	return qp, nil
}

func NewQueue(cfg *config.Queue) (*Queue, error) {
	size := cfg.Size
	return &Queue{ch: make(chan *WrapPredictRequest, size)}, nil
}

func (qp *QueuePool) Select(serviceGroup string) (queue *Queue, err error) {
	queue, ok := qp.queues[serviceGroup]
	if !ok {
		return nil, errs.ErrQueueGroupIsNotExist
	}
	return queue, nil
}

func (q *Queue) Push(ctx context.Context, in *WrapPredictRequest) {
	q.ch <- in
}

func (q *Queue) Pop(ctx context.Context) *WrapPredictRequest {
	select {
	case pr := <-q.ch:
		return pr
	case <-ctx.Done():
		return nil
	default:
		return nil
	}
}

func (q *Queue) BlockingPop(ctx context.Context) *WrapPredictRequest {
	select {
	case pr := <-q.ch:
		return pr
	case <-ctx.Done():
		return nil
	}
}

func NewRequestNotifyChan(cfg *config.Queue, sc []*config.ServiceGroup) chan struct{} {
	totalSize := int(cfg.Size) * len(sc)
	return make(chan struct{}, totalSize)
}
