package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogLifecycle interface {
	Create(obj *Catalog) error
	Remove(obj *Catalog) error
	Updated(obj *Catalog) error
}

type catalogLifecycleAdapter struct {
	lifecycle CatalogLifecycle
}

func (w *catalogLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Catalog))
}

func (w *catalogLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Catalog))
}

func (w *catalogLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Catalog))
}

func NewCatalogLifecycleAdapter(name string, client CatalogInterface, l CatalogLifecycle) CatalogHandlerFunc {
	adapter := &catalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Catalog) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
