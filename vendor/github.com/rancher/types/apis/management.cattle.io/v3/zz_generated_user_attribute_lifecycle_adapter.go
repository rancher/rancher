package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserAttributeLifecycle interface {
	Create(obj *UserAttribute) (*UserAttribute, error)
	Remove(obj *UserAttribute) (*UserAttribute, error)
	Updated(obj *UserAttribute) (*UserAttribute, error)
}

type userAttributeLifecycleAdapter struct {
	lifecycle UserAttributeLifecycle
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
	adapter := &userAttributeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *UserAttribute) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
