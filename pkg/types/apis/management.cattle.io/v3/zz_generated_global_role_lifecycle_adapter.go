package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalRoleLifecycle interface {
	Create(obj *GlobalRole) (runtime.Object, error)
	Remove(obj *GlobalRole) (runtime.Object, error)
	Updated(obj *GlobalRole) (runtime.Object, error)
}

type globalRoleLifecycleAdapter struct {
	lifecycle GlobalRoleLifecycle
}

func (w *globalRoleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *globalRoleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *globalRoleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*GlobalRole))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalRoleLifecycleAdapter(name string, clusterScoped bool, client GlobalRoleInterface, l GlobalRoleLifecycle) GlobalRoleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(GlobalRoleGroupVersionResource)
	}
	adapter := &globalRoleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *GlobalRole) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
