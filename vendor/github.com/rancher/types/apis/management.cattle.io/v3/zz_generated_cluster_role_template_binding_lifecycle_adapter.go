package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRoleTemplateBindingLifecycle interface {
	Create(obj *ClusterRoleTemplateBinding) error
	Remove(obj *ClusterRoleTemplateBinding) error
	Updated(obj *ClusterRoleTemplateBinding) error
}

type clusterRoleTemplateBindingLifecycleAdapter struct {
	lifecycle ClusterRoleTemplateBindingLifecycle
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*ClusterRoleTemplateBinding))
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*ClusterRoleTemplateBinding))
}

func (w *clusterRoleTemplateBindingLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*ClusterRoleTemplateBinding))
}

func NewClusterRoleTemplateBindingLifecycleAdapter(name string, client ClusterRoleTemplateBindingInterface, l ClusterRoleTemplateBindingLifecycle) ClusterRoleTemplateBindingHandlerFunc {
	adapter := &clusterRoleTemplateBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRoleTemplateBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
