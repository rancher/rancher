package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RoleBindingLifecycle interface {
	Create(obj *v1.RoleBinding) (runtime.Object, error)
	Remove(obj *v1.RoleBinding) (runtime.Object, error)
	Updated(obj *v1.RoleBinding) (runtime.Object, error)
}

type roleBindingLifecycleAdapter struct {
	lifecycle RoleBindingLifecycle
}

func (w *roleBindingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *roleBindingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *roleBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.RoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.RoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.RoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRoleBindingLifecycleAdapter(name string, clusterScoped bool, client RoleBindingInterface, l RoleBindingLifecycle) RoleBindingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RoleBindingGroupVersionResource)
	}
	adapter := &roleBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.RoleBinding) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
