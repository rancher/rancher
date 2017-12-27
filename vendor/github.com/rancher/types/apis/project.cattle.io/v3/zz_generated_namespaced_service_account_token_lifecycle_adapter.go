package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedServiceAccountTokenLifecycle interface {
	Create(obj *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Remove(obj *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
	Updated(obj *NamespacedServiceAccountToken) (*NamespacedServiceAccountToken, error)
}

type namespacedServiceAccountTokenLifecycleAdapter struct {
	lifecycle NamespacedServiceAccountTokenLifecycle
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedServiceAccountTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespacedServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedServiceAccountTokenLifecycleAdapter(name string, client NamespacedServiceAccountTokenInterface, l NamespacedServiceAccountTokenLifecycle) NamespacedServiceAccountTokenHandlerFunc {
	adapter := &namespacedServiceAccountTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *NamespacedServiceAccountToken) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
