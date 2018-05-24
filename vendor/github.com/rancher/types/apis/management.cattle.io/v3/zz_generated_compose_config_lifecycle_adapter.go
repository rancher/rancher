package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ComposeConfigLifecycle interface {
	Create(obj *ComposeConfig) (*ComposeConfig, error)
	Remove(obj *ComposeConfig) (*ComposeConfig, error)
	Updated(obj *ComposeConfig) (*ComposeConfig, error)
}

type composeConfigLifecycleAdapter struct {
	lifecycle ComposeConfigLifecycle
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
	adapter := &composeConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ComposeConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
