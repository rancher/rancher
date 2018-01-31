package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type LocalConfigLifecycle interface {
	Create(obj *LocalConfig) (*LocalConfig, error)
	Remove(obj *LocalConfig) (*LocalConfig, error)
	Updated(obj *LocalConfig) (*LocalConfig, error)
}

type localConfigLifecycleAdapter struct {
	lifecycle LocalConfigLifecycle
}

func (w *localConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*LocalConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *localConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*LocalConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *localConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*LocalConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewLocalConfigLifecycleAdapter(name string, clusterScoped bool, client LocalConfigInterface, l LocalConfigLifecycle) LocalConfigHandlerFunc {
	adapter := &localConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *LocalConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
