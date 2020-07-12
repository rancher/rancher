package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type SettingLifecycle interface {
	Create(obj *Setting) (runtime.Object, error)
	Remove(obj *Setting) (runtime.Object, error)
	Updated(obj *Setting) (runtime.Object, error)
}

type settingLifecycleAdapter struct {
	lifecycle SettingLifecycle
}

func (w *settingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *settingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(SettingGroupVersionResource)
	}
	adapter := &settingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Setting) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
