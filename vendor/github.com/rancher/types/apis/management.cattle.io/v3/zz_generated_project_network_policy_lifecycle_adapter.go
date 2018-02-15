package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectNetworkPolicyLifecycle interface {
	Create(obj *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	Remove(obj *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
	Updated(obj *ProjectNetworkPolicy) (*ProjectNetworkPolicy, error)
}

type projectNetworkPolicyLifecycleAdapter struct {
	lifecycle ProjectNetworkPolicyLifecycle
}

func (w *projectNetworkPolicyLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectNetworkPolicyLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectNetworkPolicyLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectNetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectNetworkPolicyLifecycleAdapter(name string, clusterScoped bool, client ProjectNetworkPolicyInterface, l ProjectNetworkPolicyLifecycle) ProjectNetworkPolicyHandlerFunc {
	adapter := &projectNetworkPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectNetworkPolicy) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
