package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type SettingLifecycle interface {
	Create(obj *Setting) (*Setting, error)
	Remove(obj *Setting) (*Setting, error)
	Updated(obj *Setting) (*Setting, error)
}

type settingLifecycleAdapter struct {
	lifecycle SettingLifecycle
}

func (w *settingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Setting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *settingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Setting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *settingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Setting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSettingLifecycleAdapter(name string, clusterScoped bool, client SettingInterface, l SettingLifecycle) SettingHandlerFunc {
	adapter := &settingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Setting) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
