package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type RkeAddonLifecycle interface {
	Create(obj *RkeAddon) (runtime.Object, error)
	Remove(obj *RkeAddon) (runtime.Object, error)
	Updated(obj *RkeAddon) (runtime.Object, error)
}

type rkeAddonLifecycleAdapter struct {
	lifecycle RkeAddonLifecycle
}

func (w *rkeAddonLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *rkeAddonLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *rkeAddonLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*RkeAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeAddonLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*RkeAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeAddonLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*RkeAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRkeAddonLifecycleAdapter(name string, clusterScoped bool, client RkeAddonInterface, l RkeAddonLifecycle) RkeAddonHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RkeAddonGroupVersionResource)
	}
	adapter := &rkeAddonLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *RkeAddon) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
