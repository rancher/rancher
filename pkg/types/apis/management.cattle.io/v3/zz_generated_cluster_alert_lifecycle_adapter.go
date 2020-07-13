package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAlertLifecycle interface {
	Create(obj *ClusterAlert) (runtime.Object, error)
	Remove(obj *ClusterAlert) (runtime.Object, error)
	Updated(obj *ClusterAlert) (runtime.Object, error)
}

type clusterAlertLifecycleAdapter struct {
	lifecycle ClusterAlertLifecycle
}

func (w *clusterAlertLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterAlertLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterAlertLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterAlertLifecycleAdapter(name string, clusterScoped bool, client ClusterAlertInterface, l ClusterAlertLifecycle) ClusterAlertHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterAlertGroupVersionResource)
	}
	adapter := &clusterAlertLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterAlert) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
