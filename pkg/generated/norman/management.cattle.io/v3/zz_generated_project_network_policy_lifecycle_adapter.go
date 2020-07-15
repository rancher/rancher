package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectNetworkPolicyLifecycle interface {
	Create(obj *v3.ProjectNetworkPolicy) (runtime.Object, error)
	Remove(obj *v3.ProjectNetworkPolicy) (runtime.Object, error)
	Updated(obj *v3.ProjectNetworkPolicy) (runtime.Object, error)
}

type projectNetworkPolicyLifecycleAdapter struct {
	lifecycle ProjectNetworkPolicyLifecycle
}

func (w *projectNetworkPolicyLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectNetworkPolicyLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectNetworkPolicyLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectNetworkPolicyLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectNetworkPolicyLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectNetworkPolicyLifecycleAdapter(name string, clusterScoped bool, client ProjectNetworkPolicyInterface, l ProjectNetworkPolicyLifecycle) ProjectNetworkPolicyHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectNetworkPolicyGroupVersionResource)
	}
	adapter := &projectNetworkPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ProjectNetworkPolicy) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
