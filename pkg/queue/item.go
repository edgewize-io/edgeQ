package queue

import (
	"context"
	"net/http"
	"time"
)

type HttpRequestItem struct {
	ServiceGroup string
	CreateTime   time.Time
	Req          *http.Request
	Writer       http.ResponseWriter
	Ctx          context.Context
	WaitChan     chan struct{}
}

func (item *HttpRequestItem) ResourceName() string {
	return item.ServiceGroup
}
