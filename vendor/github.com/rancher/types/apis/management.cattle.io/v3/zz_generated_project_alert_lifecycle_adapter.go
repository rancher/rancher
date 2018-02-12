package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectAlertLifecycle interface {
	Create(obj *ProjectAlert) (*ProjectAlert, error)
	Remove(obj *ProjectAlert) (*ProjectAlert, error)
	Updated(obj *ProjectAlert) (*ProjectAlert, error)
}

type projectAlertLifecycleAdapter struct {
	lifecycle ProjectAlertLifecycle
}

func (w *projectAlertLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectAlert))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectAlertLifecycleAdapter(name string, clusterScoped bool, client ProjectAlertInterface, l ProjectAlertLifecycle) ProjectAlertHandlerFunc {
	adapter := &projectAlertLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectAlert) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
