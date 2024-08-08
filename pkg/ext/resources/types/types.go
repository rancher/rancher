package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
)

type WatchEvent[T runtime.Object] struct {
	Event  watch.EventType
	Object T
}

// XXX: This is similar to k8s.io/apiserver/pkg/registry/rest.Standard, we could
// aim to be somewhat compatible if we ever want to switch to using that. 🤷

// XXX: This should probably take a context instead. This way we can propagate
// the context of the original request here. (user.Info could go into the
// context, but I kinda like it there)

// XXX: All functions should be given some options:
//      - Get: metav1.GetOptions
//      - List: metav1.ListOptions or metainternalversion.ListOptions
//      - Update: metav1.UpdateOptions
//      - Create: metav1.CreateOptions
//      - Delete: metav1.DeleteOptions
// Those are needed for some query params: limit, page, resourceVersion, etc.

type Store[T runtime.Object, TList runtime.Object] interface {
	Create(userInfo user.Info, obj T) (T, error)
	Update(userInfo user.Info, obj T) (T, error)
	Get(userInfo user.Info, name string) (T, error)
	List(userInfo user.Info) (TList, error)
	// XXX: There's no way to safely close the channel outside of this call.
	// We should probably return an interface instead like k8s.io/apimachinery/pkg/watch.Interface
	Watch(userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error)
	Delete(userInfo user.Info, name string) error
}

type backingStore[T runtime.Object, TList runtime.Object] struct {
	createFunc func(userInfo user.Info, obj T) (T, error)
	updateFunc func(userInfo user.Info, obj T) (T, error)
	getFunc    func(userInfo user.Info, name string) (T, error)
	listFunc   func(userInfo user.Info) (TList, error)
	watchFunc  func(userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error)
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

func (b *backingStore[T, TList]) Watch(userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error) {
	return b.watchFunc(userInfo, opts)
}

func (b *backingStore[T, TList]) Delete(userInfo user.Info, name string) error {
	return b.deleteFunc(userInfo, name)
}

func NewStore[T runtime.Object, TList runtime.Object](
	createFunc func(userInfo user.Info, obj T) (T, error),
	updateFunc func(userInfo user.Info, obj T) (T, error),
	getFunc func(userInfo user.Info, name string) (T, error),
	listFunc func(userInfo user.Info) (TList, error),
	watchFunc func(userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error),
	deleteFunc func(userInfo user.Info, name string) error,
) Store[T, TList] {
	return &backingStore[T, TList]{
		createFunc: createFunc,
		updateFunc: updateFunc,
		getFunc:    getFunc,
		listFunc:   listFunc,
		watchFunc:  watchFunc,
		deleteFunc: deleteFunc,
	}
}
