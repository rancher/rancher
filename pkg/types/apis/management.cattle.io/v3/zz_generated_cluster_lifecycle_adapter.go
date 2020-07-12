package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterLifecycle interface {
	Create(obj *Cluster) (runtime.Object, error)
	Remove(obj *Cluster) (runtime.Object, error)
	Updated(obj *Cluster) (runtime.Object, error)
}

type clusterLifecycleAdapter struct {
	lifecycle ClusterLifecycle
}

func (w *clusterLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(ClusterGroupVersionResource)
	}
	adapter := &clusterLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Cluster) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
