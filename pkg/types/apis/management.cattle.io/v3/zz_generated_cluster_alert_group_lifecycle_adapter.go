package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAlertGroupLifecycle interface {
	Create(obj *ClusterAlertGroup) (runtime.Object, error)
	Remove(obj *ClusterAlertGroup) (runtime.Object, error)
	Updated(obj *ClusterAlertGroup) (runtime.Object, error)
}

type clusterAlertGroupLifecycleAdapter struct {
	lifecycle ClusterAlertGroupLifecycle
}

func (w *clusterAlertGroupLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterAlertGroupLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterAlertGroupLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertGroupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertGroupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterAlertGroupLifecycleAdapter(name string, clusterScoped bool, client ClusterAlertGroupInterface, l ClusterAlertGroupLifecycle) ClusterAlertGroupHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterAlertGroupGroupVersionResource)
	}
	adapter := &clusterAlertGroupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterAlertGroup) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
