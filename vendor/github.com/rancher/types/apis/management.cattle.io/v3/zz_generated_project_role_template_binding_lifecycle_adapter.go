package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectRoleTemplateBindingLifecycle interface {
	Create(obj *ProjectRoleTemplateBinding) error
	Remove(obj *ProjectRoleTemplateBinding) error
	Updated(obj *ProjectRoleTemplateBinding) error
}

type projectRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ProjectRoleTemplateBindingLifecycle
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*ProjectRoleTemplateBinding))
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*ProjectRoleTemplateBinding))
}

func (w *projectRoleTemplateBindingLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*ProjectRoleTemplateBinding))
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
