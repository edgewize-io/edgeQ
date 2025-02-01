package picker

import (
	errs "github.com/edgewize/modelmesh/internal/broker/error"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/edgewize/modelmesh/internal/broker/picker/priorityqueue"
)

const recordTimes = 10

type MrtPickerBuilder struct {
}

func (pb *MrtPickerBuilder) Build(info PickerBuildInfo) Picker {
	if len(info.Resources) == 0 {
		return NewErrPicker(errs.ErrNoSubConnAvailable)
	}
	scs := []PickerKey{}
	//scToAddr := make(map[PickerKey]resolver.Address)
	scCostTime := priorityqueue.NewPriorityQueue()
	scRecords := make([][]int64, len(info.Resources))
	i := 0
	//for sc, scInfo := range info.Resources {
	for sc := range info.Resources {
		scs = append(scs, sc)
		//scToAddr[sc] = scInfo.Address
		scCostTime.PushItem(&priorityqueue.Item{
			//Addr:  scInfo.Address.Addr,
			Val:   0,
			Index: i,
		})
		scRecords[i] = make([]int64, recordTimes)
		i++
	}

	return &mrtPicker{
		subConns: scs,
		//scToAddr:   scToAddr,
		scCostTime: scCostTime,
		scRecords:  scRecords,
		// Start at a random index, as the same RR balancer rebuilds a new
		// picker when Resource states change, and we don't want to apply excess
		// load to the first server in the list.
		next: rand.Intn(len(scs)),
	}
}

type mrtPicker struct {
	// subConns is the snapshot of the roundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected Resource.
	subConns []PickerKey

	//scToAddr map[PickerKey]resolver.Address

	// subConns response cost time
	scCostTime *priorityqueue.PriorityQueue

	// last ten times response cost time
	scRecords [][]int64

	mu   sync.Mutex
	next int
}

func (p *mrtPicker) Pick(opts PickInfo) (PickResult, error) {
	p.mu.Lock()
	n := len(p.subConns)
	p.mu.Unlock()
	if n == 0 {
		return PickResult{}, errs.ErrNoSubConnAvailable
	}

	p.mu.Lock()
	item := p.scCostTime.Min().(*priorityqueue.Item)
	p.next = item.Index
	//log.Println("next", item.Index, p.scToAddr[p.subConns[p.next]].Addr, item.Val, n, item.Addr)
	sc := p.subConns[p.next]
	//picked := p.scToAddr[sc]
	p.mu.Unlock()

	start := time.Now()
	done := func(info DoneInfo) {
		sub := time.Since(start).Microseconds()
		if info.Err == nil {
			p.mu.Lock()
			p.scRecords[item.Index] = append(p.scRecords[item.Index][1:], sub)
			item.Val = weightedAverage(p.scRecords[item.Index])
			p.scCostTime.UpdateItem(item)
			p.mu.Unlock()
		}
		//log.Println("mrtpicker done", picked.Addr, p.next, sub)
	}

	return PickResult{Resource: sc, Done: done}, nil
}

func weightedAverage(vals []int64) int64 {
	var (
		min int64 = math.MaxInt64
		max int64 = math.MinInt64
		sum int64 = 0
		n   int64 = int64(len(vals))
	)

	for _, val := range vals {
		sum += val
		if min > val {
			min = val
		}
		if max < val {
			max = val
		}
	}

	if n <= 2 {
		return sum / n
	}

	sum = sum - min - max
	return sum / (n - 2)
}
