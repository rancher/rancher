package types

import (
	"context"

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

type Store[T runtime.Object, TList runtime.Object] interface {
	Create(ctx context.Context, userInfo user.Info, obj T, opts *metav1.CreateOptions) (T, error)
	Update(ctx context.Context, userInfo user.Info, obj T, opts *metav1.UpdateOptions) (T, error)
	Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (T, error)
	List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (TList, error)
	// XXX: There's no way to safely close the channel outside of this call.
	// We should probably return an interface instead like k8s.io/apimachinery/pkg/watch.Interface
	Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error)
	Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error
}

type backingStore[T runtime.Object, TList runtime.Object] struct {
	createFunc func(ctx context.Context, userInfo user.Info, obj T, opts *metav1.CreateOptions) (T, error)
	updateFunc func(ctx context.Context, userInfo user.Info, obj T, opts *metav1.UpdateOptions) (T, error)
	getFunc    func(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (T, error)
	listFunc   func(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (TList, error)
	watchFunc  func(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error)
	deleteFunc func(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error
}

func (b *backingStore[T, TList]) Create(ctx context.Context, userInfo user.Info, obj T, opts *metav1.CreateOptions) (T, error) {
	return b.createFunc(ctx, userInfo, obj, opts)
}

func (b *backingStore[T, TList]) Update(ctx context.Context, userInfo user.Info, obj T, opts *metav1.UpdateOptions) (T, error) {
	return b.updateFunc(ctx, userInfo, obj, opts)
}

func (b *backingStore[T, TList]) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (T, error) {
	return b.getFunc(ctx, userInfo, name, opts)
}

func (b *backingStore[T, TList]) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (TList, error) {
	return b.listFunc(ctx, userInfo, opts)
}

func (b *backingStore[T, TList]) Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error) {
	return b.watchFunc(ctx, userInfo, opts)
}

func (b *backingStore[T, TList]) Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error {
	return b.deleteFunc(ctx, userInfo, name, opts)
}

func NewStore[T runtime.Object, TList runtime.Object](
	createFunc func(ctx context.Context, userInfo user.Info, obj T, opts *metav1.CreateOptions) (T, error),
	updateFunc func(ctx context.Context, userInfo user.Info, obj T, opts *metav1.UpdateOptions) (T, error),
	getFunc func(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (T, error),
	listFunc func(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (TList, error),
	watchFunc func(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan WatchEvent[T], error),
	deleteFunc func(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error,
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
