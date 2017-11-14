package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type TemplateLifecycle interface {
	Create(obj *Template) error
	Remove(obj *Template) error
	Updated(obj *Template) error
}

type templateLifecycleAdapter struct {
	lifecycle TemplateLifecycle
}

func (w *templateLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Template))
}

func (w *templateLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Template))
}

func (w *templateLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Template))
}

func NewTemplateLifecycleAdapter(name string, client TemplateInterface, l TemplateLifecycle) TemplateHandlerFunc {
	adapter := &templateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Template) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
