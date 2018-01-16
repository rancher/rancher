package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRoleTemplateBindingLifecycle interface {
	Create(obj *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	Remove(obj *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
	Updated(obj *ClusterRoleTemplateBinding) (*ClusterRoleTemplateBinding, error)
}

type clusterRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ClusterRoleTemplateBindingLifecycle
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
	adapter := &clusterRoleTemplateBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRoleTemplateBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
