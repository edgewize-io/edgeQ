package proxy

import (
	"context"
	"fmt"
	v1 "github.com/edgewize/edgeQ/api/modelfulx/v1alpha"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"time"
)

type QueueSelect func() (serviceGroup string, err error)

type PredictRequest = v1.PredictRequest

type Queue struct {
	ch      chan *PredictRequest
	timeout time.Duration
}

func NewQueue(cfg *config.Queue, timeout time.Duration) (*Queue, error) {
	size := cfg.Size
	return &Queue{ch: make(chan *PredictRequest, size), timeout: timeout}, nil
}

func (q *Queue) Push(in *PredictRequest) (err error) {
	select {
	case q.ch <- in:
	case <-time.After(q.timeout):
		err = fmt.Errorf("handling request %s timeout", in.Id)
	}
	return
}

func (q *Queue) Pop(ctx context.Context) *PredictRequest {
	select {
	case pr := <-q.ch:
		return pr
	default:
		return nil
	}
}
