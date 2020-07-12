package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type CisConfigLifecycle interface {
	Create(obj *CisConfig) (runtime.Object, error)
	Remove(obj *CisConfig) (runtime.Object, error)
	Updated(obj *CisConfig) (runtime.Object, error)
}

type cisConfigLifecycleAdapter struct {
	lifecycle CisConfigLifecycle
}

func (w *cisConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *cisConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *cisConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*CisConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cisConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*CisConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cisConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*CisConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCisConfigLifecycleAdapter(name string, clusterScoped bool, client CisConfigInterface, l CisConfigLifecycle) CisConfigHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CisConfigGroupVersionResource)
	}
	adapter := &cisConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CisConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
