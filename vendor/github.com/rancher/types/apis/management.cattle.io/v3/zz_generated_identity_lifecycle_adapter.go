package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type IdentityLifecycle interface {
	Create(obj *Identity) (*Identity, error)
	Remove(obj *Identity) (*Identity, error)
	Updated(obj *Identity) (*Identity, error)
}

type identityLifecycleAdapter struct {
	lifecycle IdentityLifecycle
}

func (w *identityLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Identity))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *identityLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Identity))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *identityLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Identity))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewIdentityLifecycleAdapter(name string, client IdentityInterface, l IdentityLifecycle) IdentityHandlerFunc {
	adapter := &identityLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Identity) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
