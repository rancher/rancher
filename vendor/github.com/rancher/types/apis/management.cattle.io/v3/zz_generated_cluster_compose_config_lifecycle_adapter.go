package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterComposeConfigLifecycle interface {
	Create(obj *ClusterComposeConfig) (*ClusterComposeConfig, error)
	Remove(obj *ClusterComposeConfig) (*ClusterComposeConfig, error)
	Updated(obj *ClusterComposeConfig) (*ClusterComposeConfig, error)
}

type clusterComposeConfigLifecycleAdapter struct {
	lifecycle ClusterComposeConfigLifecycle
}

func (w *clusterComposeConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterComposeConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterComposeConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterComposeConfigLifecycleAdapter(name string, clusterScoped bool, client ClusterComposeConfigInterface, l ClusterComposeConfigLifecycle) ClusterComposeConfigHandlerFunc {
	adapter := &clusterComposeConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterComposeConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
