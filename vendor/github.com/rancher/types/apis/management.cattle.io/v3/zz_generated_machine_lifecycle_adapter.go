package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineLifecycle interface {
	Create(obj *Machine) error
	Remove(obj *Machine) error
	Updated(obj *Machine) error
}

type machineLifecycleAdapter struct {
	lifecycle MachineLifecycle
}

func (w *machineLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Machine))
}

func (w *machineLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Machine))
}

func (w *machineLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Machine))
}

func NewMachineLifecycleAdapter(name string, client MachineInterface, l MachineLifecycle) MachineHandlerFunc {
	adapter := &machineLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Machine) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
