package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventLifecycle interface {
	Create(obj *v1.Event) error
	Remove(obj *v1.Event) error
	Updated(obj *v1.Event) error
}

type eventLifecycleAdapter struct {
	lifecycle EventLifecycle
}

func (w *eventLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1.Event))
}

func (w *eventLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1.Event))
}

func (w *eventLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1.Event))
}

func NewEventLifecycleAdapter(name string, client EventInterface, l EventLifecycle) EventHandlerFunc {
	adapter := &eventLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.Event) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
