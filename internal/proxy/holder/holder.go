package holder

import (
	"context"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type holder struct {
	lock    sync.RWMutex
	holders map[string]chan Response
	timeout time.Duration
}

func New(ctx context.Context, timeout time.Duration) Holder {
	return &holder{
		lock:    sync.RWMutex{},
		holders: make(map[string]chan Response),
		timeout: timeout,
	}
}

// Cancel cancel all waiters.
func (h *holder) Cancel() {
	h.lock.Lock()
	for id, ch := range h.holders {
		ch <- Response{
			ID:         id,
			Status:     StatusCanceled,
			ErrMessage: "canceled",
		}
	}
	h.lock.Unlock()
}

func (h *holder) Wait(ctx context.Context, id string) Response {
	w := h.wait(ctx, id)
	resp := <-w.ch
	return resp
}

func (h *holder) wait(ctx context.Context, id string) *Waiter {
	h.lock.Lock()
	// chan size 为3 避免response和超时对chan的竞争.
	waitCh := make(chan Response, 2)
	h.holders[id] = waitCh
	h.lock.Unlock()

	go func() {

		var lastErrMessage string
		var lastStatus Status
		select {
		case <-ctx.Done():
			if nil != ctx.Err() {
				klog.V(2).Infof("receive context done, id: %s, %v", id, ctx.Err())
				lastStatus = StatusCanceled
				lastErrMessage = ctx.Err().Error()
			}
		case <-time.After(h.timeout):
			klog.Warningf("waiting for request id: [%s] timeout", id)
			lastStatus = StatusTimeout
			lastErrMessage = "waiting response timeout"
		}

		// delete wait channel.
		h.lock.Lock()
		delete(h.holders, id)
		h.lock.Unlock()

		waitCh <- Response{
			ID:         id,
			Status:     lastStatus,
			ErrMessage: lastErrMessage,
		}
	}()

	return &Waiter{
		ch: waitCh,
	}
}

func (h *holder) Notify(resp *Response) {
	klog.V(2).InfoS("received response", "respID", resp.ID, "status", resp.Status)

	h.lock.Lock()
	waitCh := h.holders[resp.ID]
	delete(h.holders, resp.ID)
	h.lock.Unlock()

	if nil == waitCh {
		klog.Warningf("holders request %s is not exists", resp.ID)
		return
	}

	waitCh <- *resp
}
