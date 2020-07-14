package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkloadLifecycle interface {
	Create(obj *v3.Workload) (runtime.Object, error)
	Remove(obj *v3.Workload) (runtime.Object, error)
	Updated(obj *v3.Workload) (runtime.Object, error)
}

type workloadLifecycleAdapter struct {
	lifecycle WorkloadLifecycle
}

func (w *workloadLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *workloadLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *workloadLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *workloadLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *workloadLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Workload))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewWorkloadLifecycleAdapter(name string, clusterScoped bool, client WorkloadInterface, l WorkloadLifecycle) WorkloadHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(WorkloadGroupVersionResource)
	}
	adapter := &workloadLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Workload) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
