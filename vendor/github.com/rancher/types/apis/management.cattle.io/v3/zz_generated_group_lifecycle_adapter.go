package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupLifecycle interface {
	Create(obj *Group) error
	Remove(obj *Group) error
	Updated(obj *Group) error
}

type groupLifecycleAdapter struct {
	lifecycle GroupLifecycle
}

func (w *groupLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Group))
}

func (w *groupLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Group))
}

func (w *groupLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Group))
}

func NewGroupLifecycleAdapter(name string, client GroupInterface, l GroupLifecycle) GroupHandlerFunc {
	adapter := &groupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Group) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
