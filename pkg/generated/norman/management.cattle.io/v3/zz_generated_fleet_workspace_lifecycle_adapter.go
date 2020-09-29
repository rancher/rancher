package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type FleetWorkspaceLifecycle interface {
	Create(obj *v3.FleetWorkspace) (runtime.Object, error)
	Remove(obj *v3.FleetWorkspace) (runtime.Object, error)
	Updated(obj *v3.FleetWorkspace) (runtime.Object, error)
}

type fleetWorkspaceLifecycleAdapter struct {
	lifecycle FleetWorkspaceLifecycle
}

func (w *fleetWorkspaceLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *fleetWorkspaceLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *fleetWorkspaceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.FleetWorkspace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *fleetWorkspaceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.FleetWorkspace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *fleetWorkspaceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.FleetWorkspace))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewFleetWorkspaceLifecycleAdapter(name string, clusterScoped bool, client FleetWorkspaceInterface, l FleetWorkspaceLifecycle) FleetWorkspaceHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(FleetWorkspaceGroupVersionResource)
	}
	adapter := &fleetWorkspaceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.FleetWorkspace) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
