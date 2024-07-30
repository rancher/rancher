package types

import "k8s.io/apimachinery/pkg/runtime"

type Store[T runtime.Object] interface {
	Create(obj T) (T, error)
	Update(obj T) (T, error)
	Get(name string) (T, error)
	List() ([]T, error)
	Delete(name string) error
}

type backingStore[T runtime.Object] struct {
	createFunc func(obj T) (T, error)
	updateFunc func(obj T) (T, error)
	getFunc    func(name string) (T, error)
	listFunc   func() ([]T, error)
	deleteFunc func(name string) error
}

func (b *backingStore[T]) Create(obj T) (T, error) {
	return b.createFunc(obj)
}

func (b *backingStore[T]) Update(obj T) (T, error) {
	return b.updateFunc(obj)
}

func (b *backingStore[T]) Get(name string) (T, error) {
	return b.getFunc(name)
}

func (b *backingStore[T]) List() ([]T, error) {
	return b.listFunc()
}

func (b *backingStore[T]) Delete(name string) error {
	return b.deleteFunc(name)
}

func NewStore[T runtime.Object](createFunc func(obj T) (T, error),
	updateFunc func(obj T) (T, error),
	getFunc func(name string) (T, error),
	listFunc func() ([]T, error),
	deleteFunc func(name string) error,
) Store[T] {
	return &backingStore[T]{
		createFunc: createFunc,
		updateFunc: updateFunc,
		getFunc:    getFunc,
		listFunc:   listFunc,
		deleteFunc: deleteFunc,
	}
}
