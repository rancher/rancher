package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAuthTokenLifecycle interface {
	Create(obj *v3.ClusterAuthToken) (runtime.Object, error)
	Remove(obj *v3.ClusterAuthToken) (runtime.Object, error)
	Updated(obj *v3.ClusterAuthToken) (runtime.Object, error)
}

type clusterAuthTokenLifecycleAdapter struct {
	lifecycle ClusterAuthTokenLifecycle
}

func (w *clusterAuthTokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterAuthTokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterAuthTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ClusterAuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAuthTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ClusterAuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAuthTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ClusterAuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterAuthTokenLifecycleAdapter(name string, clusterScoped bool, client ClusterAuthTokenInterface, l ClusterAuthTokenLifecycle) ClusterAuthTokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterAuthTokenGroupVersionResource)
	}
	adapter := &clusterAuthTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ClusterAuthToken) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
