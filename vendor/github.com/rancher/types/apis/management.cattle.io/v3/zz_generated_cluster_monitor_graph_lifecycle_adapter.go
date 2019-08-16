package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterMonitorGraphLifecycle interface {
	Create(obj *ClusterMonitorGraph) (runtime.Object, error)
	Remove(obj *ClusterMonitorGraph) (runtime.Object, error)
	Updated(obj *ClusterMonitorGraph) (runtime.Object, error)
}

type clusterMonitorGraphLifecycleAdapter struct {
	lifecycle ClusterMonitorGraphLifecycle
}

func (w *clusterMonitorGraphLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterMonitorGraphLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterMonitorGraphLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterMonitorGraphLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterMonitorGraphLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterMonitorGraphLifecycleAdapter(name string, clusterScoped bool, client ClusterMonitorGraphInterface, l ClusterMonitorGraphLifecycle) ClusterMonitorGraphHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterMonitorGraphGroupVersionResource)
	}
	adapter := &clusterMonitorGraphLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterMonitorGraph) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
