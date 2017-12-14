package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type MachineTemplateLifecycle interface {
	Create(obj *MachineTemplate) (*MachineTemplate, error)
	Remove(obj *MachineTemplate) (*MachineTemplate, error)
	Updated(obj *MachineTemplate) (*MachineTemplate, error)
}

type machineTemplateLifecycleAdapter struct {
	lifecycle MachineTemplateLifecycle
}

func (w *machineTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*MachineTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*MachineTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *machineTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*MachineTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
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
