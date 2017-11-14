package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkloadLifecycle interface {
	Create(obj *Workload) error
	Remove(obj *Workload) error
	Updated(obj *Workload) error
}

type workloadLifecycleAdapter struct {
	lifecycle WorkloadLifecycle
}

func (w *workloadLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Workload))
}

func (w *workloadLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Workload))
}

func (w *workloadLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Workload))
}

func NewWorkloadLifecycleAdapter(name string, client WorkloadInterface, l WorkloadLifecycle) WorkloadHandlerFunc {
	adapter := &workloadLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Workload) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
