package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedSSHAuthLifecycle interface {
	Create(obj *v3.NamespacedSSHAuth) (runtime.Object, error)
	Remove(obj *v3.NamespacedSSHAuth) (runtime.Object, error)
	Updated(obj *v3.NamespacedSSHAuth) (runtime.Object, error)
}

type namespacedSshAuthLifecycleAdapter struct {
	lifecycle NamespacedSSHAuthLifecycle
}

func (w *namespacedSshAuthLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespacedSshAuthLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespacedSshAuthLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedSshAuthLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedSshAuthLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedSSHAuthLifecycleAdapter(name string, clusterScoped bool, client NamespacedSSHAuthInterface, l NamespacedSSHAuthLifecycle) NamespacedSSHAuthHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespacedSSHAuthGroupVersionResource)
	}
	adapter := &namespacedSshAuthLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.NamespacedSSHAuth) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
