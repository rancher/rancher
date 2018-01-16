package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupLifecycle interface {
	Create(obj *Group) (*Group, error)
	Remove(obj *Group) (*Group, error)
	Updated(obj *Group) (*Group, error)
}

type groupLifecycleAdapter struct {
	lifecycle GroupLifecycle
}

func (w *groupLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGroupLifecycleAdapter(name string, clusterScoped bool, client GroupInterface, l GroupLifecycle) GroupHandlerFunc {
	adapter := &groupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Group) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
