package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterCatalogLifecycle interface {
	Create(obj *ClusterCatalog) (runtime.Object, error)
	Remove(obj *ClusterCatalog) (runtime.Object, error)
	Updated(obj *ClusterCatalog) (runtime.Object, error)
}

type clusterCatalogLifecycleAdapter struct {
	lifecycle ClusterCatalogLifecycle
}

func (w *clusterCatalogLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterCatalogLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterCatalogLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterCatalogLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterCatalogLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterCatalogLifecycleAdapter(name string, clusterScoped bool, client ClusterCatalogInterface, l ClusterCatalogLifecycle) ClusterCatalogHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterCatalogGroupVersionResource)
	}
	adapter := &clusterCatalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterCatalog) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
