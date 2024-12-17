package serviceaccounttoken

import (
	"container/list"
	"sync"
)

// newQueue is a new queue for comparables.
func newQueue[T comparable](objs ...T) *queue[T] {
	q := &queue[T]{List: list.New()}
	q.enqueue(objs...)

	return q
}

// Simple queued list
type queue[T comparable] struct {
	sync.Mutex
	*list.List
}

func (q *queue[T]) enqueue(v ...T) {
	q.Lock()
	defer q.Unlock()
	for _, vt := range v {
		q.PushBack(vt)
	}
}

func (q *queue[T]) dequeue(n int64) []T {
	q.Lock()
	defer q.Unlock()

	var result []T

	for i := n; i > 0; i-- {
		if q.Len() == 0 {
			return result
		}
		e := q.Front()
		q.List.Remove(e)
		result = append(result, e.Value.(T))
	}

	return result
}
