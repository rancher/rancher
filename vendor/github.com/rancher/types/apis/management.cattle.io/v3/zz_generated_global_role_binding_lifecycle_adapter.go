package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalRoleBindingLifecycle interface {
	Create(obj *GlobalRoleBinding) (*GlobalRoleBinding, error)
	Remove(obj *GlobalRoleBinding) (*GlobalRoleBinding, error)
	Updated(obj *GlobalRoleBinding) (*GlobalRoleBinding, error)
}

type globalRoleBindingLifecycleAdapter struct {
	lifecycle GlobalRoleBindingLifecycle
}

func (w *globalRoleBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalRoleBindingLifecycleAdapter(name string, client GlobalRoleBindingInterface, l GlobalRoleBindingLifecycle) GlobalRoleBindingHandlerFunc {
	adapter := &globalRoleBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *GlobalRoleBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
