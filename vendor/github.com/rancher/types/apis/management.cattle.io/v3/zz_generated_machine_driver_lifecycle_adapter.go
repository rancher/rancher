package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineDriverLifecycle interface {
	Create(obj *MachineDriver) (*MachineDriver, error)
	Remove(obj *MachineDriver) (*MachineDriver, error)
	Updated(obj *MachineDriver) (*MachineDriver, error)
}

type machineDriverLifecycleAdapter struct {
	lifecycle MachineDriverLifecycle
}

func (w *machineDriverLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*MachineDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineDriverLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*MachineDriver))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineDriverLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*MachineDriver))
	if o == nil {
		return nil, err
	}
	return o, err
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
