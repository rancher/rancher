package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserLifecycle interface {
	Create(obj *User) error
	Remove(obj *User) error
	Updated(obj *User) error
}

type userLifecycleAdapter struct {
	lifecycle UserLifecycle
}

func (w *userLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*User))
}

func (w *userLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*User))
}

func (w *userLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*User))
}

func NewUserLifecycleAdapter(name string, client UserInterface, l UserLifecycle) UserHandlerFunc {
	adapter := &userLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *User) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
