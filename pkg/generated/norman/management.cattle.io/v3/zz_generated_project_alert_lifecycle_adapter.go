package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectAlertLifecycle interface {
	Create(obj *v3.ProjectAlert) (runtime.Object, error)
	Remove(obj *v3.ProjectAlert) (runtime.Object, error)
	Updated(obj *v3.ProjectAlert) (runtime.Object, error)
}

type projectAlertLifecycleAdapter struct {
	lifecycle ProjectAlertLifecycle
}

func (w *projectAlertLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectAlertLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectAlertLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectAlertLifecycleAdapter(name string, clusterScoped bool, client ProjectAlertInterface, l ProjectAlertLifecycle) ProjectAlertHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectAlertGroupVersionResource)
	}
	adapter := &projectAlertLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ProjectAlert) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
