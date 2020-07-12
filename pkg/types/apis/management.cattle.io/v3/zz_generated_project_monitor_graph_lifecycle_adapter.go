package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectMonitorGraphLifecycle interface {
	Create(obj *ProjectMonitorGraph) (runtime.Object, error)
	Remove(obj *ProjectMonitorGraph) (runtime.Object, error)
	Updated(obj *ProjectMonitorGraph) (runtime.Object, error)
}

type projectMonitorGraphLifecycleAdapter struct {
	lifecycle ProjectMonitorGraphLifecycle
}

func (w *projectMonitorGraphLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectMonitorGraphLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectMonitorGraphLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectMonitorGraphLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectMonitorGraphLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectMonitorGraph))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectMonitorGraphLifecycleAdapter(name string, clusterScoped bool, client ProjectMonitorGraphInterface, l ProjectMonitorGraphLifecycle) ProjectMonitorGraphHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectMonitorGraphGroupVersionResource)
	}
	adapter := &projectMonitorGraphLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectMonitorGraph) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
