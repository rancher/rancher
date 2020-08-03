package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRoleLifecycle interface {
	Create(obj *v1.ClusterRole) (runtime.Object, error)
	Remove(obj *v1.ClusterRole) (runtime.Object, error)
	Updated(obj *v1.ClusterRole) (runtime.Object, error)
}

type clusterRoleLifecycleAdapter struct {
	lifecycle ClusterRoleLifecycle
}

func (w *clusterRoleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterRoleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterRoleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ClusterRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ClusterRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ClusterRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRoleLifecycleAdapter(name string, clusterScoped bool, client ClusterRoleInterface, l ClusterRoleLifecycle) ClusterRoleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterRoleGroupVersionResource)
	}
	adapter := &clusterRoleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ClusterRole) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
