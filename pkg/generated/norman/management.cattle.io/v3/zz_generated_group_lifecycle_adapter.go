package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type GroupLifecycle interface {
	Create(obj *v3.Group) (runtime.Object, error)
	Remove(obj *v3.Group) (runtime.Object, error)
	Updated(obj *v3.Group) (runtime.Object, error)
}

type groupLifecycleAdapter struct {
	lifecycle GroupLifecycle
}

func (w *groupLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *groupLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *groupLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *groupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Group))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGroupLifecycleAdapter(name string, clusterScoped bool, client GroupInterface, l GroupLifecycle) GroupHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(GroupGroupVersionResource)
	}
	adapter := &groupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Group) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
