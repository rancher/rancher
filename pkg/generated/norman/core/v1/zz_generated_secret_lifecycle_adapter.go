package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SecretLifecycle interface {
	Create(obj *v1.Secret) (runtime.Object, error)
	Remove(obj *v1.Secret) (runtime.Object, error)
	Updated(obj *v1.Secret) (runtime.Object, error)
}

type secretLifecycleAdapter struct {
	lifecycle SecretLifecycle
}

func (w *secretLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *secretLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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

func NewSecretLifecycleAdapter(name string, clusterScoped bool, client SecretInterface, l SecretLifecycle) SecretHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(SecretGroupVersionResource)
	}
	adapter := &secretLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Secret) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
