package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PersistentVolumeClaimLifecycle interface {
	Create(obj *v1.PersistentVolumeClaim) (runtime.Object, error)
	Remove(obj *v1.PersistentVolumeClaim) (runtime.Object, error)
	Updated(obj *v1.PersistentVolumeClaim) (runtime.Object, error)
}

type persistentVolumeClaimLifecycleAdapter struct {
	lifecycle PersistentVolumeClaimLifecycle
}

func (w *persistentVolumeClaimLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *persistentVolumeClaimLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *persistentVolumeClaimLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.PersistentVolumeClaim))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *persistentVolumeClaimLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.PersistentVolumeClaim))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *persistentVolumeClaimLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.PersistentVolumeClaim))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPersistentVolumeClaimLifecycleAdapter(name string, clusterScoped bool, client PersistentVolumeClaimInterface, l PersistentVolumeClaimLifecycle) PersistentVolumeClaimHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PersistentVolumeClaimGroupVersionResource)
	}
	adapter := &persistentVolumeClaimLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.PersistentVolumeClaim) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
