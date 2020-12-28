package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/cluster.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterUserAttributeLifecycle interface {
	Create(obj *v3.ClusterUserAttribute) (runtime.Object, error)
	Remove(obj *v3.ClusterUserAttribute) (runtime.Object, error)
	Updated(obj *v3.ClusterUserAttribute) (runtime.Object, error)
}

type clusterUserAttributeLifecycleAdapter struct {
	lifecycle ClusterUserAttributeLifecycle
}

func (w *clusterUserAttributeLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterUserAttributeLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterUserAttributeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ClusterUserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterUserAttributeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ClusterUserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterUserAttributeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ClusterUserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterUserAttributeLifecycleAdapter(name string, clusterScoped bool, client ClusterUserAttributeInterface, l ClusterUserAttributeLifecycle) ClusterUserAttributeHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterUserAttributeGroupVersionResource)
	}
	adapter := &clusterUserAttributeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ClusterUserAttribute) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
