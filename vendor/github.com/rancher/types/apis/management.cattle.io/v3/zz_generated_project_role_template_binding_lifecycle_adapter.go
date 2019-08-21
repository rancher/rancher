package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectRoleTemplateBindingLifecycle interface {
	Create(obj *ProjectRoleTemplateBinding) (runtime.Object, error)
	Remove(obj *ProjectRoleTemplateBinding) (runtime.Object, error)
	Updated(obj *ProjectRoleTemplateBinding) (runtime.Object, error)
}

type projectRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ProjectRoleTemplateBindingLifecycle
}

func (w *projectRoleTemplateBindingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectRoleTemplateBindingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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

func NewProjectRoleTemplateBindingLifecycleAdapter(name string, clusterScoped bool, client ProjectRoleTemplateBindingInterface, l ProjectRoleTemplateBindingLifecycle) ProjectRoleTemplateBindingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectRoleTemplateBindingGroupVersionResource)
	}
	adapter := &projectRoleTemplateBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectRoleTemplateBinding) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
