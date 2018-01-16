package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterLifecycle interface {
	Create(obj *Cluster) (*Cluster, error)
	Remove(obj *Cluster) (*Cluster, error)
	Updated(obj *Cluster) (*Cluster, error)
}

type clusterLifecycleAdapter struct {
	lifecycle ClusterLifecycle
}

func (w *clusterLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Cluster))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Cluster))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Cluster))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterLifecycleAdapter(name string, clusterScoped bool, client ClusterInterface, l ClusterLifecycle) ClusterHandlerFunc {
	adapter := &clusterLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Cluster) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
