package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedBasicAuthLifecycle interface {
	Create(obj *NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	Remove(obj *NamespacedBasicAuth) (*NamespacedBasicAuth, error)
	Updated(obj *NamespacedBasicAuth) (*NamespacedBasicAuth, error)
}

type namespacedBasicAuthLifecycleAdapter struct {
	lifecycle NamespacedBasicAuthLifecycle
}

func (w *namespacedBasicAuthLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedBasicAuthLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedBasicAuthLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedBasicAuthLifecycleAdapter(name string, clusterScoped bool, client NamespacedBasicAuthInterface, l NamespacedBasicAuthLifecycle) NamespacedBasicAuthHandlerFunc {
	adapter := &namespacedBasicAuthLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NamespacedBasicAuth) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
