package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SecretLifecycle interface {
	Create(obj *v1.Secret) (*v1.Secret, error)
	Remove(obj *v1.Secret) (*v1.Secret, error)
	Updated(obj *v1.Secret) (*v1.Secret, error)
}

type secretLifecycleAdapter struct {
	lifecycle SecretLifecycle
}

func (w *secretLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Secret))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *secretLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Secret))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *secretLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Secret))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSecretLifecycleAdapter(name string, client SecretInterface, l SecretLifecycle) SecretHandlerFunc {
	adapter := &secretLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.Secret) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
