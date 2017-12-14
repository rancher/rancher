package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type RoleTemplateLifecycle interface {
	Create(obj *RoleTemplate) (*RoleTemplate, error)
	Remove(obj *RoleTemplate) (*RoleTemplate, error)
	Updated(obj *RoleTemplate) (*RoleTemplate, error)
}

type roleTemplateLifecycleAdapter struct {
	lifecycle RoleTemplateLifecycle
}

func (w *roleTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRoleTemplateLifecycleAdapter(name string, client RoleTemplateInterface, l RoleTemplateLifecycle) RoleTemplateHandlerFunc {
	adapter := &roleTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *RoleTemplate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
