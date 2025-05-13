package queue

import (
	"context"
	"net/http"
	"time"
)

type HTTPRequestItem struct {
	ServiceGroup string
	CreateTime   time.Time
	Req          *http.Request
	Writer       http.ResponseWriter
	Ctx          context.Context
	WaitChan     chan struct{}
}

func (h *HTTPRequestItem) ResourceName() string {
	return h.ServiceGroup
}

func (h *HTTPRequestItem) SetResourceName(name string) {
	h.ServiceGroup = name
}

func (h *HTTPRequestItem) GetCreateTime() time.Time {
	return h.CreateTime
}
