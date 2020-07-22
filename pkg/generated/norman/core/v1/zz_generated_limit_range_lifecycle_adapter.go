package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type LimitRangeLifecycle interface {
	Create(obj *v1.LimitRange) (runtime.Object, error)
	Remove(obj *v1.LimitRange) (runtime.Object, error)
	Updated(obj *v1.LimitRange) (runtime.Object, error)
}

type limitRangeLifecycleAdapter struct {
	lifecycle LimitRangeLifecycle
}

func (w *limitRangeLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *limitRangeLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *limitRangeLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.LimitRange))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *limitRangeLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.LimitRange))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *limitRangeLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.LimitRange))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewLimitRangeLifecycleAdapter(name string, clusterScoped bool, client LimitRangeInterface, l LimitRangeLifecycle) LimitRangeHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(LimitRangeGroupVersionResource)
	}
	adapter := &limitRangeLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.LimitRange) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
