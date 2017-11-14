package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type TemplateVersionLifecycle interface {
	Create(obj *TemplateVersion) error
	Remove(obj *TemplateVersion) error
	Updated(obj *TemplateVersion) error
}

type templateVersionLifecycleAdapter struct {
	lifecycle TemplateVersionLifecycle
}

func (w *templateVersionLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*TemplateVersion))
}

func (w *templateVersionLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*TemplateVersion))
}

func (w *templateVersionLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*TemplateVersion))
}

func NewTemplateVersionLifecycleAdapter(name string, client TemplateVersionInterface, l TemplateVersionLifecycle) TemplateVersionHandlerFunc {
	adapter := &templateVersionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *TemplateVersion) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
