package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ListenConfigLifecycle interface {
	Create(obj *ListenConfig) (runtime.Object, error)
	Remove(obj *ListenConfig) (runtime.Object, error)
	Updated(obj *ListenConfig) (runtime.Object, error)
}

type listenConfigLifecycleAdapter struct {
	lifecycle ListenConfigLifecycle
}

func (w *listenConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *listenConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *listenConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ListenConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *listenConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ListenConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *listenConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ListenConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewListenConfigLifecycleAdapter(name string, clusterScoped bool, client ListenConfigInterface, l ListenConfigLifecycle) ListenConfigHandlerFunc {
	adapter := &listenConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ListenConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
