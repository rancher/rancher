package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RoleLifecycle interface {
	Create(obj *v1.Role) (runtime.Object, error)
	Remove(obj *v1.Role) (runtime.Object, error)
	Updated(obj *v1.Role) (runtime.Object, error)
}

type roleLifecycleAdapter struct {
	lifecycle RoleLifecycle
}

func (w *roleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *roleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *roleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Role))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Role))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *roleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Role))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRoleLifecycleAdapter(name string, clusterScoped bool, client RoleInterface, l RoleLifecycle) RoleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RoleGroupVersionResource)
	}
	adapter := &roleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Role) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
