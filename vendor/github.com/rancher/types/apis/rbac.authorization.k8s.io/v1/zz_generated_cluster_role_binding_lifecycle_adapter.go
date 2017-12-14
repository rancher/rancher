package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRoleBindingLifecycle interface {
	Create(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Remove(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
	Updated(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error)
}

type clusterRoleBindingLifecycleAdapter struct {
	lifecycle ClusterRoleBindingLifecycle
}

func (w *clusterRoleBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ClusterRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ClusterRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRoleBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ClusterRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRoleBindingLifecycleAdapter(name string, client ClusterRoleBindingInterface, l ClusterRoleBindingLifecycle) ClusterRoleBindingHandlerFunc {
	adapter := &clusterRoleBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.ClusterRoleBinding) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
