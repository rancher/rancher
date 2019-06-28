package v1

import (
	"github.com/rancher/norman/lifecycle"
	v1 "github.com/rancher/terraform-controller/pkg/apis/terraformcontroller.cattle.io/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ModuleLifecycle interface {
	Create(obj *v1.Module) (runtime.Object, error)
	Remove(obj *v1.Module) (runtime.Object, error)
	Updated(obj *v1.Module) (runtime.Object, error)
}

type moduleLifecycleAdapter struct {
	lifecycle ModuleLifecycle
}

func (w *moduleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *moduleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *moduleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Module))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *moduleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Module))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *moduleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Module))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewModuleLifecycleAdapter(name string, clusterScoped bool, client ModuleInterface, l ModuleLifecycle) ModuleHandlerFunc {
	adapter := &moduleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Module) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
