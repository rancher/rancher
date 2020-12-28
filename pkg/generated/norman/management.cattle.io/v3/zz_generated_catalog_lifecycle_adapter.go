package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogLifecycle interface {
	Create(obj *v3.Catalog) (runtime.Object, error)
	Remove(obj *v3.Catalog) (runtime.Object, error)
	Updated(obj *v3.Catalog) (runtime.Object, error)
}

type catalogLifecycleAdapter struct {
	lifecycle CatalogLifecycle
}

func (w *catalogLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *catalogLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *catalogLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCatalogLifecycleAdapter(name string, clusterScoped bool, client CatalogInterface, l CatalogLifecycle) CatalogHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CatalogGroupVersionResource)
	}
	adapter := &catalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Catalog) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
