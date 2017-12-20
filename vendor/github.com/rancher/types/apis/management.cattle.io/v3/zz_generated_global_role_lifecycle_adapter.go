package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalRoleLifecycle interface {
	Create(obj *GlobalRole) (*GlobalRole, error)
	Remove(obj *GlobalRole) (*GlobalRole, error)
	Updated(obj *GlobalRole) (*GlobalRole, error)
}

type globalRoleLifecycleAdapter struct {
	lifecycle GlobalRoleLifecycle
}

func (w *globalRoleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalRoleLifecycleAdapter(name string, client GlobalRoleInterface, l GlobalRoleLifecycle) GlobalRoleHandlerFunc {
	adapter := &globalRoleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *GlobalRole) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
