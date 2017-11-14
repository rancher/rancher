package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ComponentStatusLifecycle interface {
	Create(obj *v1.ComponentStatus) error
	Remove(obj *v1.ComponentStatus) error
	Updated(obj *v1.ComponentStatus) error
}

type componentStatusLifecycleAdapter struct {
	lifecycle ComponentStatusLifecycle
}

func (w *componentStatusLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1.ComponentStatus))
}

func (w *componentStatusLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1.ComponentStatus))
}

func (w *componentStatusLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1.ComponentStatus))
}

func NewComponentStatusLifecycleAdapter(name string, client ComponentStatusInterface, l ComponentStatusLifecycle) ComponentStatusHandlerFunc {
	adapter := &componentStatusLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.ComponentStatus) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
