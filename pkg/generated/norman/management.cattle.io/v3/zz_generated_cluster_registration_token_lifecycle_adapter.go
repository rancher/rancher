package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRegistrationTokenLifecycle interface {
	Create(obj *v3.ClusterRegistrationToken) (runtime.Object, error)
	Remove(obj *v3.ClusterRegistrationToken) (runtime.Object, error)
	Updated(obj *v3.ClusterRegistrationToken) (runtime.Object, error)
}

type clusterRegistrationTokenLifecycleAdapter struct {
	lifecycle ClusterRegistrationTokenLifecycle
}

func (w *clusterRegistrationTokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterRegistrationTokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterRegistrationTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRegistrationTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRegistrationTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ClusterRegistrationToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRegistrationTokenLifecycleAdapter(name string, clusterScoped bool, client ClusterRegistrationTokenInterface, l ClusterRegistrationTokenLifecycle) ClusterRegistrationTokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterRegistrationTokenGroupVersionResource)
	}
	adapter := &clusterRegistrationTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ClusterRegistrationToken) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
