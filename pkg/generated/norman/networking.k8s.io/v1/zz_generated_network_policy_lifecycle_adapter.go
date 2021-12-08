package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NetworkPolicyLifecycle interface {
	Create(obj *v1.NetworkPolicy) (runtime.Object, error)
	Remove(obj *v1.NetworkPolicy) (runtime.Object, error)
	Updated(obj *v1.NetworkPolicy) (runtime.Object, error)
}

type networkPolicyLifecycleAdapter struct {
	lifecycle NetworkPolicyLifecycle
}

func (w *networkPolicyLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *networkPolicyLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *networkPolicyLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.NetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *networkPolicyLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.NetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *networkPolicyLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.NetworkPolicy))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNetworkPolicyLifecycleAdapter(name string, clusterScoped bool, client NetworkPolicyInterface, l NetworkPolicyLifecycle) NetworkPolicyHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NetworkPolicyGroupVersionResource)
	}
	adapter := &networkPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.NetworkPolicy) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
