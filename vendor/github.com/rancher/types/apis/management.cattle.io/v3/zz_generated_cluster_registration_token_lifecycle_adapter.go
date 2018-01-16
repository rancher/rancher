package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRegistrationTokenLifecycle interface {
	Create(obj *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	Remove(obj *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	Updated(obj *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
}

type clusterRegistrationTokenLifecycleAdapter struct {
	lifecycle ClusterRegistrationTokenLifecycle
}

func (w *clusterRegistrationTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRegistrationTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRegistrationTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRegistrationTokenLifecycleAdapter(name string, clusterScoped bool, client ClusterRegistrationTokenInterface, l ClusterRegistrationTokenLifecycle) ClusterRegistrationTokenHandlerFunc {
	adapter := &clusterRegistrationTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRegistrationToken) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
