package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineDriverLifecycle interface {
	Create(obj *MachineDriver) error
	Remove(obj *MachineDriver) error
	Updated(obj *MachineDriver) error
}

type machineDriverLifecycleAdapter struct {
	lifecycle MachineDriverLifecycle
}

func (w *machineDriverLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*MachineDriver))
}

func (w *machineDriverLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*MachineDriver))
}

func (w *machineDriverLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*MachineDriver))
}

func NewMachineDriverLifecycleAdapter(name string, client MachineDriverInterface, l MachineDriverLifecycle) MachineDriverHandlerFunc {
	adapter := &machineDriverLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *MachineDriver) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
