package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ComposeConfigLifecycle interface {
	Create(obj *ComposeConfig) (runtime.Object, error)
	Remove(obj *ComposeConfig) (runtime.Object, error)
	Updated(obj *ComposeConfig) (runtime.Object, error)
}

type composeConfigLifecycleAdapter struct {
	lifecycle ComposeConfigLifecycle
}

func (w *composeConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *composeConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *composeConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *composeConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *composeConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewComposeConfigLifecycleAdapter(name string, clusterScoped bool, client ComposeConfigInterface, l ComposeConfigLifecycle) ComposeConfigHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ComposeConfigGroupVersionResource)
	}
	adapter := &composeConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ComposeConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
