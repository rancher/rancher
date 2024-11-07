package serviceaccounttoken

import (
	"container/list"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

// newQueue is a new queue for runtime.Objects.
func newQueue[T runtime.Object](objs ...T) *queue[T] {
	q := &queue[T]{List: list.New()}
	q.enqueue(objs...)

	return q
}

// Simple queued list
type queue[T runtime.Object] struct {
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
		e := q.Front()
		if q.Len() == 0 {
			return result
		}
		q.List.Remove(e)
		result = append(result, e.Value.(T))
	}

	return result
}
