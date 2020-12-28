package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type UserLifecycle interface {
	Create(obj *v3.User) (runtime.Object, error)
	Remove(obj *v3.User) (runtime.Object, error)
	Updated(obj *v3.User) (runtime.Object, error)
}

type userLifecycleAdapter struct {
	lifecycle UserLifecycle
}

func (w *userLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *userLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *userLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *userLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.User))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewUserLifecycleAdapter(name string, clusterScoped bool, client UserInterface, l UserLifecycle) UserHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(UserGroupVersionResource)
	}
	adapter := &userLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.User) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
