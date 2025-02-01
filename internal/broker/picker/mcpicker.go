package picker

import (
	errs "github.com/edgewize/modelmesh/internal/broker/error"
	"github.com/edgewize/modelmesh/internal/broker/picker/priorityqueue"
	"math/rand"
	"sync"
)

type McPickerBuilder struct {
}

func (pb *McPickerBuilder) Build(info PickerBuildInfo) Picker {
	if len(info.Resources) == 0 {
		return NewErrPicker(errs.ErrNoSubConnAvailable)
	}
	scs := []PickerKey{}
	//scToAddr := make(map[PickerKey]resolver.Address)
	scConnectNum := priorityqueue.NewPriorityQueue()
	i := 0
	//for sc, scInfo := range info.Resources {
	for sc := range info.Resources {
		scs = append(scs, sc)
		//scToAddr[sc] = scInfo.Address
		scConnectNum.PushItem(&priorityqueue.Item{
			//Addr:  scInfo.Address.Addr,
			Val:   0,
			Index: i,
		})
		i++
	}

	return &mcPicker{
		subConns: scs,
		//scToAddr:     scToAddr,
		scConnectNum: scConnectNum,
		// Start at a random index, as the same RR balancer rebuilds a new
		// picker when Resource states change, and we don't want to apply excess
		// load to the first server in the list.
		next: rand.Intn(len(scs)),
	}
}

type mcPicker struct {
	// subConns is the snapshot of the roundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected Resource.
	subConns []PickerKey

	//scToAddr map[PickerKey]resolver.Address

	// subConns connect number
	scConnectNum *priorityqueue.PriorityQueue

	mu   sync.Mutex
	next int
}

func (p *mcPicker) Pick(opts PickInfo) (PickResult, error) {
	p.mu.Lock()
	n := len(p.subConns)
	p.mu.Unlock()
	if n == 0 {
		return PickResult{}, errs.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	sc := p.subConns[p.next]
	//picked := p.scToAddr[sc]
	item := p.scConnectNum.Min().(*priorityqueue.Item)
	p.next = item.Index
	item.Val++
	p.scConnectNum.UpdateItem(item)
	p.mu.Unlock()

	done := func(info DoneInfo) {
		p.mu.Lock()
		item.Val--
		p.scConnectNum.UpdateItem(item)
		p.mu.Unlock()
		//log.Println("mcpicker done", picked.Addr, p.next)
	}

	return PickResult{Resource: sc, Done: done}, nil
}
