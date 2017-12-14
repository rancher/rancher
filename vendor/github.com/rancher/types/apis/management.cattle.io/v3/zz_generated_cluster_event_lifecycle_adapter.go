package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterEventLifecycle interface {
	Create(obj *ClusterEvent) (*ClusterEvent, error)
	Remove(obj *ClusterEvent) (*ClusterEvent, error)
	Updated(obj *ClusterEvent) (*ClusterEvent, error)
}

type clusterEventLifecycleAdapter struct {
	lifecycle ClusterEventLifecycle
}

func (w *clusterEventLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterEvent))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterEventLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterEvent))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterEventLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterEvent))
	if o == nil {
		return nil, err
	}
	return o, err
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
