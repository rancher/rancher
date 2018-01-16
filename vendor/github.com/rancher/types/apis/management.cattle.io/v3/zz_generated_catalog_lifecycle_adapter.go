package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogLifecycle interface {
	Create(obj *Catalog) (*Catalog, error)
	Remove(obj *Catalog) (*Catalog, error)
	Updated(obj *Catalog) (*Catalog, error)
}

type catalogLifecycleAdapter struct {
	lifecycle CatalogLifecycle
}

func (w *catalogLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Catalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCatalogLifecycleAdapter(name string, clusterScoped bool, client CatalogInterface, l CatalogLifecycle) CatalogHandlerFunc {
	adapter := &catalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Catalog) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
