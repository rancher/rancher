package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type TemplateLifecycle interface {
	Create(obj *v3.Template) (runtime.Object, error)
	Remove(obj *v3.Template) (runtime.Object, error)
	Updated(obj *v3.Template) (runtime.Object, error)
}

type templateLifecycleAdapter struct {
	lifecycle TemplateLifecycle
}

func (w *templateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *templateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *templateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Template))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *templateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Template))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *templateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Template))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewTemplateLifecycleAdapter(name string, clusterScoped bool, client TemplateInterface, l TemplateLifecycle) TemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(TemplateGroupVersionResource)
	}
	adapter := &templateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Template) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
