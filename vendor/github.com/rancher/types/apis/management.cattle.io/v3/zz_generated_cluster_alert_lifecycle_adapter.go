package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAlertLifecycle interface {
	Create(obj *ClusterAlert) (*ClusterAlert, error)
	Remove(obj *ClusterAlert) (*ClusterAlert, error)
	Updated(obj *ClusterAlert) (*ClusterAlert, error)
}

type clusterAlertLifecycleAdapter struct {
	lifecycle ClusterAlertLifecycle
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
	adapter := &clusterAlertLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterAlert) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
