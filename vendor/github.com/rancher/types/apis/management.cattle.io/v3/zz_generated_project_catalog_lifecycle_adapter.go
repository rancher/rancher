package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectCatalogLifecycle interface {
	Create(obj *ProjectCatalog) (*ProjectCatalog, error)
	Remove(obj *ProjectCatalog) (*ProjectCatalog, error)
	Updated(obj *ProjectCatalog) (*ProjectCatalog, error)
}

type projectCatalogLifecycleAdapter struct {
	lifecycle ProjectCatalogLifecycle
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
	adapter := &projectCatalogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectCatalog) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
