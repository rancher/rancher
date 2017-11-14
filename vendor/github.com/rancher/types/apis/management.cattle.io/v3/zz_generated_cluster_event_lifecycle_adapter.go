package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterEventLifecycle interface {
	Create(obj *ClusterEvent) error
	Remove(obj *ClusterEvent) error
	Updated(obj *ClusterEvent) error
}

type clusterEventLifecycleAdapter struct {
	lifecycle ClusterEventLifecycle
}

func (w *clusterEventLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*ClusterEvent))
}

func (w *clusterEventLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*ClusterEvent))
}

func (w *clusterEventLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*ClusterEvent))
}

func NewClusterEventLifecycleAdapter(name string, client ClusterEventInterface, l ClusterEventLifecycle) ClusterEventHandlerFunc {
	adapter := &clusterEventLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *ClusterEvent) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
