package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkloadLifecycle interface {
	Create(obj *Workload) (*Workload, error)
	Remove(obj *Workload) (*Workload, error)
	Updated(obj *Workload) (*Workload, error)
}

type workloadLifecycleAdapter struct {
	lifecycle WorkloadLifecycle
}

func (w *workloadLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *workloadLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *workloadLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewWorkloadLifecycleAdapter(name string, clusterScoped bool, client WorkloadInterface, l WorkloadLifecycle) WorkloadHandlerFunc {
	adapter := &workloadLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Workload) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
