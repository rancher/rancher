package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogTemplateLifecycle interface {
	Create(obj *CatalogTemplate) (runtime.Object, error)
	Remove(obj *CatalogTemplate) (runtime.Object, error)
	Updated(obj *CatalogTemplate) (runtime.Object, error)
}

type catalogTemplateLifecycleAdapter struct {
	lifecycle CatalogTemplateLifecycle
}

func (w *catalogTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *catalogTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *catalogTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*CatalogTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*CatalogTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*CatalogTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCatalogTemplateLifecycleAdapter(name string, clusterScoped bool, client CatalogTemplateInterface, l CatalogTemplateLifecycle) CatalogTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CatalogTemplateGroupVersionResource)
	}
	adapter := &catalogTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CatalogTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
