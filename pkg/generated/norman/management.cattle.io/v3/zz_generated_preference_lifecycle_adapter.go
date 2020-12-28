package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type PreferenceLifecycle interface {
	Create(obj *v3.Preference) (runtime.Object, error)
	Remove(obj *v3.Preference) (runtime.Object, error)
	Updated(obj *v3.Preference) (runtime.Object, error)
}

type preferenceLifecycleAdapter struct {
	lifecycle PreferenceLifecycle
}

func (w *preferenceLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *preferenceLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *preferenceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Preference))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *preferenceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Preference))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *preferenceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Preference))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPreferenceLifecycleAdapter(name string, clusterScoped bool, client PreferenceInterface, l PreferenceLifecycle) PreferenceHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PreferenceGroupVersionResource)
	}
	adapter := &preferenceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Preference) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
