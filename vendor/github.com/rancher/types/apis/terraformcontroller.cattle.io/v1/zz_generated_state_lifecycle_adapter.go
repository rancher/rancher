package v1

import (
	"github.com/rancher/norman/lifecycle"
	v1 "github.com/rancher/terraform-controller/pkg/apis/terraformcontroller.cattle.io/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type StateLifecycle interface {
	Create(obj *v1.State) (runtime.Object, error)
	Remove(obj *v1.State) (runtime.Object, error)
	Updated(obj *v1.State) (runtime.Object, error)
}

type stateLifecycleAdapter struct {
	lifecycle StateLifecycle
}

func (w *stateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *stateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *stateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.State))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *stateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.State))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *stateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.State))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewStateLifecycleAdapter(name string, clusterScoped bool, client StateInterface, l StateLifecycle) StateHandlerFunc {
	adapter := &stateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.State) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
