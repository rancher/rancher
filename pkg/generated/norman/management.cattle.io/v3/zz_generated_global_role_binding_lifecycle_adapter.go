package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlobalRoleBindingLifecycle interface {
	Create(obj *v3.GlobalRoleBinding) (runtime.Object, error)
	Remove(obj *v3.GlobalRoleBinding) (runtime.Object, error)
	Updated(obj *v3.GlobalRoleBinding) (runtime.Object, error)
}

type globalRoleBindingLifecycleAdapter struct {
	lifecycle GlobalRoleBindingLifecycle
}

func (w *globalRoleBindingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *globalRoleBindingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *globalRoleBindingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleBindingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *globalRoleBindingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.GlobalRoleBinding))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewGlobalRoleBindingLifecycleAdapter(name string, clusterScoped bool, client GlobalRoleBindingInterface, l GlobalRoleBindingLifecycle) GlobalRoleBindingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(GlobalRoleBindingGroupVersionResource)
	}
	adapter := &globalRoleBindingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
