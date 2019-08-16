package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogTemplateVersionLifecycle interface {
	Create(obj *CatalogTemplateVersion) (runtime.Object, error)
	Remove(obj *CatalogTemplateVersion) (runtime.Object, error)
	Updated(obj *CatalogTemplateVersion) (runtime.Object, error)
}

type catalogTemplateVersionLifecycleAdapter struct {
	lifecycle CatalogTemplateVersionLifecycle
}

func (w *catalogTemplateVersionLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *catalogTemplateVersionLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *catalogTemplateVersionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*CatalogTemplateVersion))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateVersionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*CatalogTemplateVersion))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateVersionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*CatalogTemplateVersion))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCatalogTemplateVersionLifecycleAdapter(name string, clusterScoped bool, client CatalogTemplateVersionInterface, l CatalogTemplateVersionLifecycle) CatalogTemplateVersionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CatalogTemplateVersionGroupVersionResource)
	}
	adapter := &catalogTemplateVersionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CatalogTemplateVersion) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
