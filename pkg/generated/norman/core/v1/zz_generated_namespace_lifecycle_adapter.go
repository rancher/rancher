package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespaceLifecycle interface {
	Create(obj *v1.Namespace) (runtime.Object, error)
	Remove(obj *v1.Namespace) (runtime.Object, error)
	Updated(obj *v1.Namespace) (runtime.Object, error)
}

type namespaceLifecycleAdapter struct {
	lifecycle NamespaceLifecycle
}

func (w *namespaceLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespaceLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespaceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Namespace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespaceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Namespace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespaceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Namespace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespaceLifecycleAdapter(name string, clusterScoped bool, client NamespaceInterface, l NamespaceLifecycle) NamespaceHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespaceGroupVersionResource)
	}
	adapter := &namespaceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Namespace) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
