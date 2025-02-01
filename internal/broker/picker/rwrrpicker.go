package picker

import (
	errs "github.com/edgewize/modelmesh/internal/broker/error"
	"math/rand"
	"sync"
	"time"
)

type RWRRPickerBuilder struct{}

func (pb *RWRRPickerBuilder) Build(info PickerBuildInfo) Picker {
	if len(info.Resources) == 0 {
		return NewErrPicker(errs.ErrNoSubConnAvailable)
	}

	scs := []PickerKey{}
	//scToAddr := make(map[PickerKey]resolver.Address)
	rwrr := newRwrr()
	var weight int32
	for sc, scInfo := range info.Resources {
		scs = append(scs, sc)
		//scToAddr[sc] = scInfo.Address
		weight = 1
		if scInfo.Attributes != nil {
			val := scInfo.Attributes.Value(WeightAttributeKey)
			weight = val.(int32)
		}
		rwrr.add(weight)
	}

	return &rwrrPicker{
		subConns: scs,
		//scToAddr: scToAddr,
		// Start at a random index, as the same WRR balancer rebuilds a new
		// picker when Resource states change, and we don't want to apply excess
		// load to the first server in the list.
		next: rwrr.next(),
		rwrr: rwrr,
	}
}

type rwrrPicker struct {
	// subConns is the snapshot of the weightedroundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected Resource.
	subConns []PickerKey

	//scToAddr map[PickerKey]resolver.Address

	mu sync.Mutex

	rwrr *rwrr
	next int
}

func (p *rwrrPicker) Pick(opts PickInfo) (PickResult, error) {
	p.mu.Lock()
	n := len(p.subConns)
	p.mu.Unlock()
	if n == 0 {
		return PickResult{}, errs.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	sc := p.subConns[p.next]
	p.next = p.rwrr.next()
	p.mu.Unlock()
	return PickResult{Resource: sc}, nil
}

type rwrr struct {
	weights      []int32
	sumOfWeights int32
	n            int
	rand         *rand.Rand
}

func newRwrr() *rwrr {
	return &rwrr{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r *rwrr) next() int {
	if r.n == 0 || r.sumOfWeights == 0 {
		return 0
	}

	randomWeight := r.rand.Int31n(r.sumOfWeights) + 1
	for i, weight := range r.weights {
		randomWeight -= weight
		if randomWeight <= 0 {
			return i
		}
	}

	return r.rand.Intn(r.n)
}

func (r *rwrr) add(weight int32) {
	r.weights = append(r.weights, weight)
	r.sumOfWeights += weight
	r.n++
}
