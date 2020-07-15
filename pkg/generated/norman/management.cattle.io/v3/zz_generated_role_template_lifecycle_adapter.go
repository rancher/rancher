package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type RoleTemplateLifecycle interface {
	Create(obj *v3.RoleTemplate) (runtime.Object, error)
	Remove(obj *v3.RoleTemplate) (runtime.Object, error)
	Updated(obj *v3.RoleTemplate) (runtime.Object, error)
}

type roleTemplateLifecycleAdapter struct {
	lifecycle RoleTemplateLifecycle
}

func (w *roleTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *roleTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *roleTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.RoleTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRoleTemplateLifecycleAdapter(name string, clusterScoped bool, client RoleTemplateInterface, l RoleTemplateLifecycle) RoleTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RoleTemplateGroupVersionResource)
	}
	adapter := &roleTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.RoleTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
