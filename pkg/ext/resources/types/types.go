package types

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
)

type Store[T runtime.Object, TList runtime.Object] interface {
	Create(userInfo user.Info, obj T) (T, error)
	Update(userInfo user.Info, obj T) (T, error)
	Get(userInfo user.Info, name string) (T, error)
	List(userInfo user.Info) (TList, error)
	Delete(userInfo user.Info, name string) error
}

type backingStore[T runtime.Object, TList runtime.Object] struct {
	createFunc func(userInfo user.Info, obj T) (T, error)
	updateFunc func(userInfo user.Info, obj T) (T, error)
	getFunc    func(userInfo user.Info, name string) (T, error)
	listFunc   func(userInfo user.Info) (TList, error)
	deleteFunc func(userInfo user.Info, name string) error
}

func (b *backingStore[T, TList]) Create(userInfo user.Info, obj T) (T, error) {
	return b.createFunc(userInfo, obj)
}

func (b *backingStore[T, TList]) Update(userInfo user.Info, obj T) (T, error) {
	return b.updateFunc(userInfo, obj)
}

func (b *backingStore[T, TList]) Get(userInfo user.Info, name string) (T, error) {
	return b.getFunc(userInfo, name)
}

func (b *backingStore[T, TList]) List(userInfo user.Info) (TList, error) {
	return b.listFunc(userInfo)
}

func (b *backingStore[T, TList]) Delete(userInfo user.Info, name string) error {
	return b.deleteFunc(userInfo, name)
}

func NewStore[T runtime.Object, TList runtime.Object](
	createFunc func(userInfo user.Info, obj T) (T, error),
	updateFunc func(userInfo user.Info, obj T) (T, error),
	getFunc func(userInfo user.Info, name string) (T, error),
	listFunc func(userInfo user.Info) (TList, error),
	deleteFunc func(userInfo user.Info, name string) error,
) Store[T, TList] {
	return &backingStore[T, TList]{
		createFunc: createFunc,
		updateFunc: updateFunc,
		getFunc:    getFunc,
		listFunc:   listFunc,
		deleteFunc: deleteFunc,
	}
}
