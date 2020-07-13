package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectAlertGroupLifecycle interface {
	Create(obj *ProjectAlertGroup) (runtime.Object, error)
	Remove(obj *ProjectAlertGroup) (runtime.Object, error)
	Updated(obj *ProjectAlertGroup) (runtime.Object, error)
}

type projectAlertGroupLifecycleAdapter struct {
	lifecycle ProjectAlertGroupLifecycle
}

func (w *projectAlertGroupLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectAlertGroupLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectAlertGroupLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertGroupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertGroupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectAlertGroupLifecycleAdapter(name string, clusterScoped bool, client ProjectAlertGroupInterface, l ProjectAlertGroupLifecycle) ProjectAlertGroupHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectAlertGroupGroupVersionResource)
	}
	adapter := &projectAlertGroupLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectAlertGroup) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
