package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type RKEAddonLifecycle interface {
	Create(obj *RKEAddon) (runtime.Object, error)
	Remove(obj *RKEAddon) (runtime.Object, error)
	Updated(obj *RKEAddon) (runtime.Object, error)
}

type rkeAddonLifecycleAdapter struct {
	lifecycle RKEAddonLifecycle
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
	o, err := w.lifecycle.Create(obj.(*RKEAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeAddonLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*RKEAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeAddonLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*RKEAddon))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRKEAddonLifecycleAdapter(name string, clusterScoped bool, client RKEAddonInterface, l RKEAddonLifecycle) RKEAddonHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RKEAddonGroupVersionResource)
	}
	adapter := &rkeAddonLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *RKEAddon) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
