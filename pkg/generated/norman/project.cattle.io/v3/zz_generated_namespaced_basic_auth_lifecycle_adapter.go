package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedBasicAuthLifecycle interface {
	Create(obj *v3.NamespacedBasicAuth) (runtime.Object, error)
	Remove(obj *v3.NamespacedBasicAuth) (runtime.Object, error)
	Updated(obj *v3.NamespacedBasicAuth) (runtime.Object, error)
}

type namespacedBasicAuthLifecycleAdapter struct {
	lifecycle NamespacedBasicAuthLifecycle
}

func (w *namespacedBasicAuthLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespacedBasicAuthLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespacedBasicAuthLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedBasicAuthLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedBasicAuthLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.NamespacedBasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedBasicAuthLifecycleAdapter(name string, clusterScoped bool, client NamespacedBasicAuthInterface, l NamespacedBasicAuthLifecycle) NamespacedBasicAuthHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespacedBasicAuthGroupVersionResource)
	}
	adapter := &namespacedBasicAuthLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.NamespacedBasicAuth) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
