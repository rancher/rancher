package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterScanLifecycle interface {
	Create(obj *ClusterScan) (runtime.Object, error)
	Remove(obj *ClusterScan) (runtime.Object, error)
	Updated(obj *ClusterScan) (runtime.Object, error)
}

type clusterScanLifecycleAdapter struct {
	lifecycle ClusterScanLifecycle
}

func (w *clusterScanLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterScanLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterScanLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterScan))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterScanLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterScan))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterScanLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterScan))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterScanLifecycleAdapter(name string, clusterScoped bool, client ClusterScanInterface, l ClusterScanLifecycle) ClusterScanHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterScanGroupVersionResource)
	}
	adapter := &clusterScanLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterScan) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
