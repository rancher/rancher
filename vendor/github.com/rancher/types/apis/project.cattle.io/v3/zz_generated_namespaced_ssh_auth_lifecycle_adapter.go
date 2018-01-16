package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedSSHAuthLifecycle interface {
	Create(obj *NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Remove(obj *NamespacedSSHAuth) (*NamespacedSSHAuth, error)
	Updated(obj *NamespacedSSHAuth) (*NamespacedSSHAuth, error)
}

type namespacedSshAuthLifecycleAdapter struct {
	lifecycle NamespacedSSHAuthLifecycle
}

func (w *namespacedSshAuthLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedSshAuthLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedSshAuthLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespacedSSHAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedSSHAuthLifecycleAdapter(name string, clusterScoped bool, client NamespacedSSHAuthInterface, l NamespacedSSHAuthLifecycle) NamespacedSSHAuthHandlerFunc {
	adapter := &namespacedSshAuthLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NamespacedSSHAuth) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
