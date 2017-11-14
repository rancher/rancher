package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type IdentityLifecycle interface {
	Create(obj *Identity) error
	Remove(obj *Identity) error
	Updated(obj *Identity) error
}

type identityLifecycleAdapter struct {
	lifecycle IdentityLifecycle
}

func (w *identityLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Identity))
}

func (w *identityLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Identity))
}

func (w *identityLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Identity))
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
