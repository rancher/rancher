package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectLoggingLifecycle interface {
	Create(obj *ProjectLogging) (*ProjectLogging, error)
	Remove(obj *ProjectLogging) (*ProjectLogging, error)
	Updated(obj *ProjectLogging) (*ProjectLogging, error)
}

type projectLoggingLifecycleAdapter struct {
	lifecycle ProjectLoggingLifecycle
}

func (w *projectLoggingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectLogging))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLoggingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectLogging))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLoggingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectLogging))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectLoggingLifecycleAdapter(name string, clusterScoped bool, client ProjectLoggingInterface, l ProjectLoggingLifecycle) ProjectLoggingHandlerFunc {
	adapter := &projectLoggingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectLogging) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
