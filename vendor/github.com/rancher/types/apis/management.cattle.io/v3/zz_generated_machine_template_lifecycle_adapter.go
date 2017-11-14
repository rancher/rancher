package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineTemplateLifecycle interface {
	Create(obj *MachineTemplate) error
	Remove(obj *MachineTemplate) error
	Updated(obj *MachineTemplate) error
}

type machineTemplateLifecycleAdapter struct {
	lifecycle MachineTemplateLifecycle
}

func (w *machineTemplateLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*MachineTemplate))
}

func (w *machineTemplateLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*MachineTemplate))
}

func (w *machineTemplateLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*MachineTemplate))
}

func NewMachineTemplateLifecycleAdapter(name string, client MachineTemplateInterface, l MachineTemplateLifecycle) MachineTemplateHandlerFunc {
	adapter := &machineTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *MachineTemplate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
