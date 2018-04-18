package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type TemplateContentLifecycle interface {
	Create(obj *TemplateContent) (*TemplateContent, error)
	Remove(obj *TemplateContent) (*TemplateContent, error)
	Updated(obj *TemplateContent) (*TemplateContent, error)
}

type templateContentLifecycleAdapter struct {
	lifecycle TemplateContentLifecycle
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
	adapter := &templateContentLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *TemplateContent) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
