package holder

import (
	"context"
)

type Status string

var (
	// ctx canceled or timeout
	StatusCanceled Status = "canceled"
	StatusOK       Status = "ok"
	StatusFailed   Status = "failed"
	StatusTimeout  Status = "timeout"
)

type Holder interface {
	Cancel()
	Wait(ctx context.Context, id string) Response
	Notify(*Response)
}

type Response struct {
	ID         string
	Status     Status
	ErrMessage string
	Metadata   map[string]string
	Data       interface{}
}

type Waiter struct {
	ch chan Response
}

func (w *Waiter) Wait() Response {
	resp := <-w.ch
	return resp
}
