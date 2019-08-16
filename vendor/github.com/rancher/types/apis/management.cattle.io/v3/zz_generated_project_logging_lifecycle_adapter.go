package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectLoggingLifecycle interface {
	Create(obj *ProjectLogging) (runtime.Object, error)
	Remove(obj *ProjectLogging) (runtime.Object, error)
	Updated(obj *ProjectLogging) (runtime.Object, error)
}

type projectLoggingLifecycleAdapter struct {
	lifecycle ProjectLoggingLifecycle
}

func (w *projectLoggingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectLoggingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(ProjectLoggingGroupVersionResource)
	}
	adapter := &projectLoggingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectLogging) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
