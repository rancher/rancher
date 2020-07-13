package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type TemplateContentLifecycle interface {
	Create(obj *TemplateContent) (runtime.Object, error)
	Remove(obj *TemplateContent) (runtime.Object, error)
	Updated(obj *TemplateContent) (runtime.Object, error)
}

type templateContentLifecycleAdapter struct {
	lifecycle TemplateContentLifecycle
}

func (w *templateContentLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *templateContentLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *templateContentLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*TemplateContent))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *templateContentLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*TemplateContent))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *templateContentLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*TemplateContent))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewTemplateContentLifecycleAdapter(name string, clusterScoped bool, client TemplateContentInterface, l TemplateContentLifecycle) TemplateContentHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(TemplateContentGroupVersionResource)
	}
	adapter := &templateContentLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *TemplateContent) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
