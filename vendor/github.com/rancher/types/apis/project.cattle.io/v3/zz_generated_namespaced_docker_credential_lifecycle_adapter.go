package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedDockerCredentialLifecycle interface {
	Create(obj *NamespacedDockerCredential) (*NamespacedDockerCredential, error)
	Remove(obj *NamespacedDockerCredential) (*NamespacedDockerCredential, error)
	Updated(obj *NamespacedDockerCredential) (*NamespacedDockerCredential, error)
}

type namespacedDockerCredentialLifecycleAdapter struct {
	lifecycle NamespacedDockerCredentialLifecycle
}

func (w *namespacedDockerCredentialLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedDockerCredentialLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedDockerCredentialLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedDockerCredentialLifecycleAdapter(name string, client NamespacedDockerCredentialInterface, l NamespacedDockerCredentialLifecycle) NamespacedDockerCredentialHandlerFunc {
	adapter := &namespacedDockerCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *NamespacedDockerCredential) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
