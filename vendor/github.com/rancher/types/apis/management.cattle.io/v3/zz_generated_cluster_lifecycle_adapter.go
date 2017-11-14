package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterLifecycle interface {
	Create(obj *Cluster) error
	Remove(obj *Cluster) error
	Updated(obj *Cluster) error
}

type clusterLifecycleAdapter struct {
	lifecycle ClusterLifecycle
}

func (w *clusterLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Cluster))
}

func (w *clusterLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Cluster))
}

func (w *clusterLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Cluster))
}

func NewClusterLifecycleAdapter(name string, client ClusterInterface, l ClusterLifecycle) ClusterHandlerFunc {
	adapter := &clusterLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Cluster) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
