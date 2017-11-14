package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRegistrationTokenLifecycle interface {
	Create(obj *ClusterRegistrationToken) error
	Remove(obj *ClusterRegistrationToken) error
	Updated(obj *ClusterRegistrationToken) error
}

type clusterRegistrationTokenLifecycleAdapter struct {
	lifecycle ClusterRegistrationTokenLifecycle
}

func (w *clusterRegistrationTokenLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*ClusterRegistrationToken))
}

func (w *clusterRegistrationTokenLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*ClusterRegistrationToken))
}

func (w *clusterRegistrationTokenLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*ClusterRegistrationToken))
}

func NewClusterRegistrationTokenLifecycleAdapter(name string, client ClusterRegistrationTokenInterface, l ClusterRegistrationTokenLifecycle) ClusterRegistrationTokenHandlerFunc {
	adapter := &clusterRegistrationTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRegistrationToken) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
