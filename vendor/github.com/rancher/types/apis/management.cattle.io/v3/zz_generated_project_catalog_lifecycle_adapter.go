package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectCatalogLifecycle interface {
	Create(obj *ProjectCatalog) (runtime.Object, error)
	Remove(obj *ProjectCatalog) (runtime.Object, error)
	Updated(obj *ProjectCatalog) (runtime.Object, error)
}

type projectCatalogLifecycleAdapter struct {
	lifecycle ProjectCatalogLifecycle
}

func (w *projectCatalogLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectCatalogLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectCatalogLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectCatalogLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectCatalogLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectCatalog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectCatalogLifecycleAdapter(name string, clusterScoped bool, client ProjectCatalogInterface, l ProjectCatalogLifecycle) ProjectCatalogHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectCatalogGroupVersionResource)
	}
	adapter := &projectCatalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectCatalog) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
