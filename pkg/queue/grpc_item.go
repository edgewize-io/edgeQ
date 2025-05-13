package queue

import (
	"time"
)

type GRPCItem struct {
	ServiceGroup string
	CreateTime   time.Time
	WaitChan     chan struct{}
}

func (g *GRPCItem) ResourceName() string {
	return g.ServiceGroup
}

func (g *GRPCItem) SetResourceName(name string) {
	g.ServiceGroup = name
}

func (g *GRPCItem) GetCreateTime() time.Time {
	return g.CreateTime
}
