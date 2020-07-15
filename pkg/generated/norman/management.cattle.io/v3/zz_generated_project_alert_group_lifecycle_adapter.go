package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectAlertGroupLifecycle interface {
	Create(obj *v3.ProjectAlertGroup) (runtime.Object, error)
	Remove(obj *v3.ProjectAlertGroup) (runtime.Object, error)
	Updated(obj *v3.ProjectAlertGroup) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.ProjectAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertGroupLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ProjectAlertGroup))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertGroupLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ProjectAlertGroup))
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
	return func(key string, obj *v3.ProjectAlertGroup) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
