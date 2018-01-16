package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserLifecycle interface {
	Create(obj *User) (*User, error)
	Remove(obj *User) (*User, error)
	Updated(obj *User) (*User, error)
}

type userLifecycleAdapter struct {
	lifecycle UserLifecycle
}

func (w *userLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewUserLifecycleAdapter(name string, clusterScoped bool, client UserInterface, l UserLifecycle) UserHandlerFunc {
	adapter := &userLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *User) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
