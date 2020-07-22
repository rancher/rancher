package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventLifecycle interface {
	Create(obj *v1.Event) (runtime.Object, error)
	Remove(obj *v1.Event) (runtime.Object, error)
	Updated(obj *v1.Event) (runtime.Object, error)
}

type eventLifecycleAdapter struct {
	lifecycle EventLifecycle
}

func (w *eventLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *eventLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *eventLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Event))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *eventLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Event))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *eventLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Event))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewEventLifecycleAdapter(name string, clusterScoped bool, client EventInterface, l EventLifecycle) EventHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(EventGroupVersionResource)
	}
	adapter := &eventLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Event) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
