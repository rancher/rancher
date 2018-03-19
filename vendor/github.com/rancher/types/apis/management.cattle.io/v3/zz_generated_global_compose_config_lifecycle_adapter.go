package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalComposeConfigLifecycle interface {
	Create(obj *GlobalComposeConfig) (*GlobalComposeConfig, error)
	Remove(obj *GlobalComposeConfig) (*GlobalComposeConfig, error)
	Updated(obj *GlobalComposeConfig) (*GlobalComposeConfig, error)
}

type globalComposeConfigLifecycleAdapter struct {
	lifecycle GlobalComposeConfigLifecycle
}

func (w *globalComposeConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GlobalComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalComposeConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GlobalComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalComposeConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GlobalComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalComposeConfigLifecycleAdapter(name string, clusterScoped bool, client GlobalComposeConfigInterface, l GlobalComposeConfigLifecycle) GlobalComposeConfigHandlerFunc {
	adapter := &globalComposeConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *GlobalComposeConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
