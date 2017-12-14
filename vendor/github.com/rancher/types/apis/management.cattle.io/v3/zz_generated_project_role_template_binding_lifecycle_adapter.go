package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectRoleTemplateBindingLifecycle interface {
	Create(obj *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	Remove(obj *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
	Updated(obj *ProjectRoleTemplateBinding) (*ProjectRoleTemplateBinding, error)
}

type projectRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ProjectRoleTemplateBindingLifecycle
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectRoleTemplateBindingLifecycleAdapter(name string, client ProjectRoleTemplateBindingInterface, l ProjectRoleTemplateBindingLifecycle) ProjectRoleTemplateBindingHandlerFunc {
	adapter := &projectRoleTemplateBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *ProjectRoleTemplateBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
