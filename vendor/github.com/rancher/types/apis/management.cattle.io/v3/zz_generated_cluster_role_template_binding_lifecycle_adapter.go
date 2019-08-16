package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRoleTemplateBindingLifecycle interface {
	Create(obj *ClusterRoleTemplateBinding) (runtime.Object, error)
	Remove(obj *ClusterRoleTemplateBinding) (runtime.Object, error)
	Updated(obj *ClusterRoleTemplateBinding) (runtime.Object, error)
}

type clusterRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ClusterRoleTemplateBindingLifecycle
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterRoleTemplateBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRoleTemplateBindingLifecycleAdapter(name string, clusterScoped bool, client ClusterRoleTemplateBindingInterface, l ClusterRoleTemplateBindingLifecycle) ClusterRoleTemplateBindingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterRoleTemplateBindingGroupVersionResource)
	}
	adapter := &clusterRoleTemplateBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRoleTemplateBinding) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
