package queue

import (
	"container/list"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/config"
	errs "github.com/edgewize/edgeQ/internal/broker/error"
	"github.com/edgewize/edgeQ/internal/broker/picker"
	"github.com/edgewize/edgeQ/pkg/constants"
	"sync"
)

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

func (qp *QueuePool) Push(serviceGroup string, item *HttpRequestItem) (err error) {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	queue, ok := qp.Queues[serviceGroup]
	if !ok {
		err = errs.ErrQueueGroupIsNotExist
		return
	}

	err = queue.Push(item)
	return
}

func (qp *QueuePool) Pop(p picker.Picker) (resultItem *HttpRequestItem, err error) {
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

func (q *Queue) Push(value interface{}) (err error) {
	if q.list.Len() >= q.maxSize {
		err = fmt.Errorf("queue is full")
		return
	}

	q.list.PushBack(value)
	return
}

func (q *Queue) Pop() *HttpRequestItem {
	if q.list.Len() == 0 {
		return nil
	}

	front := q.list.Front()
	q.list.Remove(front)
	return front.Value.(*HttpRequestItem)
}

func (q *Queue) Peek() *HttpRequestItem {
	if q.list.Len() == 0 {
		return nil
	}

	return q.list.Front().Value.(*HttpRequestItem)
}

func (q *Queue) Len() int {
	return q.list.Len()
}
