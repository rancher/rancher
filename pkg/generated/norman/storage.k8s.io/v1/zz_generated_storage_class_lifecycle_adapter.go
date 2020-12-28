package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type StorageClassLifecycle interface {
	Create(obj *v1.StorageClass) (runtime.Object, error)
	Remove(obj *v1.StorageClass) (runtime.Object, error)
	Updated(obj *v1.StorageClass) (runtime.Object, error)
}

type storageClassLifecycleAdapter struct {
	lifecycle StorageClassLifecycle
}

func (w *storageClassLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *storageClassLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *storageClassLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.StorageClass))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *storageClassLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.StorageClass))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *storageClassLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.StorageClass))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewStorageClassLifecycleAdapter(name string, clusterScoped bool, client StorageClassInterface, l StorageClassLifecycle) StorageClassHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(StorageClassGroupVersionResource)
	}
	adapter := &storageClassLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.StorageClass) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
