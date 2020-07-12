package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserAttributeLifecycle interface {
	Create(obj *UserAttribute) (runtime.Object, error)
	Remove(obj *UserAttribute) (runtime.Object, error)
	Updated(obj *UserAttribute) (runtime.Object, error)
}

type userAttributeLifecycleAdapter struct {
	lifecycle UserAttributeLifecycle
}

func (w *userAttributeLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *userAttributeLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *userAttributeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*UserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userAttributeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*UserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userAttributeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*UserAttribute))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewUserAttributeLifecycleAdapter(name string, clusterScoped bool, client UserAttributeInterface, l UserAttributeLifecycle) UserAttributeHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(UserAttributeGroupVersionResource)
	}
	adapter := &userAttributeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *UserAttribute) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
