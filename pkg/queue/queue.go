package queue

import (
	"container/list"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	"github.com/edgewize/edgeQ/pkg/constants"
	"sync"
	"time"
)

type QueueItem interface {
	picker.Resource
	SetResourceName(name string)
	GetCreateTime() time.Time
}

type QueuePool struct {
	Queues map[string]*Queue
	mu     sync.Mutex
}

type Queue struct {
	maxSize int
	list    *list.List
}

func NewQueuePool(cfg config.Queue, sc []config.ServiceGroup) *QueuePool {
	qp := &QueuePool{
		Queues: map[string]*Queue{},
	}

	defaultExist := false
	for _, group := range sc {
		if group.Name == constants.DefaultServiceGroup {
			defaultExist = true
		}

		qp.Queues[group.Name] = NewQueue(cfg)
	}

	if !defaultExist {
		qp.Queues[constants.DefaultServiceGroup] = NewQueue(cfg)
	}
	return qp
}

func NewQueue(cfg config.Queue) *Queue {
	return &Queue{
		maxSize: cfg.Size,
		list:    list.New(),
	}
}

func (qp *QueuePool) Push(serviceGroup string, item QueueItem) (err error) {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	queue, ok := qp.Queues[serviceGroup]
	if !ok {
		item.SetResourceName(constants.DefaultServiceGroup)
		queue = qp.Queues[constants.DefaultServiceGroup]
	}

	err = queue.Push(item)
	return
}

func (qp *QueuePool) Pop(p picker.Picker) (resultItem QueueItem, err error) {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	pickInfo := picker.PickInfo{}

	for _, queue := range qp.Queues {
		item := queue.Peek()
		if nil == item {
			continue
		}

		pickInfo.Candidates = append(pickInfo.Candidates, item)
	}

	if len(pickInfo.Candidates) == 0 {
		return
	}

	pickResult, err := p.Pick(pickInfo)
	if err != nil {
		return
	}

	resultItem = qp.Queues[pickResult.ResourceName()].Pop()
	return
}

func (q *Queue) Push(value QueueItem) (err error) {
	if q.list.Len() >= q.maxSize {
		err = fmt.Errorf("queue is full")
		return
	}

	q.list.PushBack(value)
	return
}

func (q *Queue) Pop() QueueItem {
	if q.list.Len() == 0 {
		return nil
	}

	front := q.list.Front()
	q.list.Remove(front)
	return front.Value.(QueueItem)
}

func (q *Queue) Peek() QueueItem {
	if q.list.Len() == 0 {
		return nil
	}

	return q.list.Front().Value.(QueueItem)
}

func (q *Queue) Len() int {
	return q.list.Len()
}
