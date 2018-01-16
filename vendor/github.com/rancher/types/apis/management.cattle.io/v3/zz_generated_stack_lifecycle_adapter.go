package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type StackLifecycle interface {
	Create(obj *Stack) (*Stack, error)
	Remove(obj *Stack) (*Stack, error)
	Updated(obj *Stack) (*Stack, error)
}

type stackLifecycleAdapter struct {
	lifecycle StackLifecycle
}

func (w *stackLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Stack))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *stackLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Stack))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *stackLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Stack))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewStackLifecycleAdapter(name string, clusterScoped bool, client StackInterface, l StackLifecycle) StackHandlerFunc {
	adapter := &stackLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Stack) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
