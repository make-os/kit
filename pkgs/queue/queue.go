package queue

import (
	"container/list"
	"sync"
)

// Item represents a queue item
type Item interface {
	GetID() interface{}
}

// UniqueQueue provides a queue that only allows
// unique items to be appended to it.
type UniqueQueue struct {
	sync.RWMutex
	q     *list.List
	index map[interface{}]struct{}
}

// NewUnique creates an instance of UniqueQueue
func NewUnique() *UniqueQueue {
	return &UniqueQueue{
		q:     list.New(),
		index: make(map[interface{}]struct{}),
	}
}

// Head get an item from the head of the queue
func (q *UniqueQueue) Head() Item {
	q.Lock()
	defer q.Unlock()

	el := q.q.Front()
	if el == nil {
		return nil
	}

	val := el.Value.(Item)
	delete(q.index, val.GetID())
	q.q.Remove(el)
	return val
}

// Append appends an item to the queue.
// If the item already exist, nothing is added.
func (q *UniqueQueue) Append(i Item) {
	if q.Has(i) {
		return
	}
	q.Lock()
	defer q.Unlock()
	q.q.PushBack(i)
	q.index[i.GetID()] = struct{}{}
}

// Empty checks whether the queue is empty
func (q *UniqueQueue) Empty() bool {
	return q.q.Len() == 0
}

// Has checks whether a item exist in the queue
func (q *UniqueQueue) Has(i Item) bool {
	q.RLock()
	defer q.RUnlock()
	_, ok := q.index[i.GetID()]
	return ok
}

// Size returns the size of the queue
func (q *UniqueQueue) Size() int {
	q.RLock()
	defer q.RUnlock()
	return q.q.Len()
}
