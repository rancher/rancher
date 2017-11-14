package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespaceLifecycle interface {
	Create(obj *v1.Namespace) error
	Remove(obj *v1.Namespace) error
	Updated(obj *v1.Namespace) error
}

type namespaceLifecycleAdapter struct {
	lifecycle NamespaceLifecycle
}

func (w *namespaceLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1.Namespace))
}

func (w *namespaceLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1.Namespace))
}

func (w *namespaceLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1.Namespace))
}

func NewNamespaceLifecycleAdapter(name string, client NamespaceInterface, l NamespaceLifecycle) NamespaceHandlerFunc {
	adapter := &namespaceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.Namespace) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
