package v1beta2

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
)

type StatefulSetLifecycle interface {
	Create(obj *v1beta2.StatefulSet) (runtime.Object, error)
	Remove(obj *v1beta2.StatefulSet) (runtime.Object, error)
	Updated(obj *v1beta2.StatefulSet) (runtime.Object, error)
}

type statefulSetLifecycleAdapter struct {
	lifecycle StatefulSetLifecycle
}

func (w *statefulSetLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *statefulSetLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *statefulSetLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1beta2.StatefulSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *statefulSetLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1beta2.StatefulSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *statefulSetLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1beta2.StatefulSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewStatefulSetLifecycleAdapter(name string, clusterScoped bool, client StatefulSetInterface, l StatefulSetLifecycle) StatefulSetHandlerFunc {
	adapter := &statefulSetLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta2.StatefulSet) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
