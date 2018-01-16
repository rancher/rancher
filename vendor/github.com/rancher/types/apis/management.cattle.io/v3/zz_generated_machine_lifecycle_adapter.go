package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineLifecycle interface {
	Create(obj *Machine) (*Machine, error)
	Remove(obj *Machine) (*Machine, error)
	Updated(obj *Machine) (*Machine, error)
}

type machineLifecycleAdapter struct {
	lifecycle MachineLifecycle
}

func (w *machineLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Machine))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Machine))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Machine))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewMachineLifecycleAdapter(name string, clusterScoped bool, client MachineInterface, l MachineLifecycle) MachineHandlerFunc {
	adapter := &machineLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Machine) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
