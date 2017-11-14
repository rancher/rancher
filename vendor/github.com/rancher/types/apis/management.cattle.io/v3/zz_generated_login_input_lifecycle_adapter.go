package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type LoginInputLifecycle interface {
	Create(obj *LoginInput) error
	Remove(obj *LoginInput) error
	Updated(obj *LoginInput) error
}

type loginInputLifecycleAdapter struct {
	lifecycle LoginInputLifecycle
}

func (w *loginInputLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*LoginInput))
}

func (w *loginInputLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*LoginInput))
}

func (w *loginInputLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*LoginInput))
}

func NewLoginInputLifecycleAdapter(name string, client LoginInputInterface, l LoginInputLifecycle) LoginInputHandlerFunc {
	adapter := &loginInputLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *LoginInput) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
