package v1beta2

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
)

type StatefulSetLifecycle interface {
	Create(obj *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
	Remove(obj *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
	Updated(obj *v1beta2.StatefulSet) (*v1beta2.StatefulSet, error)
}

type statefulSetLifecycleAdapter struct {
	lifecycle StatefulSetLifecycle
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
	return func(key string, obj *v1beta2.StatefulSet) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
