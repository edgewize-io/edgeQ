package picker

import (
	errs "github.com/edgewize/edgeQ/internal/broker/error"
	"google.golang.org/grpc/balancer"
	"math/rand"
	"sync"
)

type RRPickerBuilder struct{}

func (pb *RRPickerBuilder) Build(info *PickerBuildInfo) Picker {
	if len(info.Resources) == 0 {
		return NewErrPicker(errs.ErrNoSubConnAvailable)
	}
	scs := make([]PickerKey, 0)

	for sc := range info.Resources {
		scs = append(scs, sc)
	}

	return &rrPicker{
		subConns: scs,
		next:     rand.Intn(len(scs)),
	}
}

type rrPicker struct {
	subConns []PickerKey
	mu       sync.Mutex
	next     int
}

func (p *rrPicker) Pick(opts PickInfo) (PickResult, error) {
	if len(opts.Candidates) == 0 {
		return PickResult{}, balancer.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	n := len(p.subConns)
	p.mu.Unlock()
	if n == 0 {
		return PickResult{}, balancer.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	currNext := p.next % len(opts.Candidates)
	sc := opts.Candidates[currNext]
	p.next = (p.next + 1) % len(p.subConns)
	p.mu.Unlock()

	return PickResult{Resource: sc}, nil
}
