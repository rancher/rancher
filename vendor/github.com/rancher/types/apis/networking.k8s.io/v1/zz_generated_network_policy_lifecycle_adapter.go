package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type NetworkPolicyLifecycle interface {
	Create(obj *v1.NetworkPolicy) (*v1.NetworkPolicy, error)
	Remove(obj *v1.NetworkPolicy) (*v1.NetworkPolicy, error)
	Updated(obj *v1.NetworkPolicy) (*v1.NetworkPolicy, error)
}

type networkPolicyLifecycleAdapter struct {
	lifecycle NetworkPolicyLifecycle
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
	adapter := &networkPolicyLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.NetworkPolicy) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
