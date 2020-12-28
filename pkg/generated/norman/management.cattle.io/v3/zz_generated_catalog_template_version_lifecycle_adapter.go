package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type CatalogTemplateVersionLifecycle interface {
	Create(obj *v3.CatalogTemplateVersion) (runtime.Object, error)
	Remove(obj *v3.CatalogTemplateVersion) (runtime.Object, error)
	Updated(obj *v3.CatalogTemplateVersion) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.CatalogTemplateVersion))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateVersionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.CatalogTemplateVersion))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *catalogTemplateVersionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.CatalogTemplateVersion))
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
	return func(key string, obj *v3.CatalogTemplateVersion) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
