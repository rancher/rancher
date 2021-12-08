package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedServiceAccountTokenLifecycle interface {
	Create(obj *v3.NamespacedServiceAccountToken) (runtime.Object, error)
	Remove(obj *v3.NamespacedServiceAccountToken) (runtime.Object, error)
	Updated(obj *v3.NamespacedServiceAccountToken) (runtime.Object, error)
}

type namespacedServiceAccountTokenLifecycleAdapter struct {
	lifecycle NamespacedServiceAccountTokenLifecycle
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedServiceAccountTokenLifecycleAdapter(name string, clusterScoped bool, client NamespacedServiceAccountTokenInterface, l NamespacedServiceAccountTokenLifecycle) NamespacedServiceAccountTokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespacedServiceAccountTokenGroupVersionResource)
	}
	adapter := &namespacedServiceAccountTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.NamespacedServiceAccountToken) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
