package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PersistentVolumeClaimLifecycle interface {
	Create(obj *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Remove(obj *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
	Updated(obj *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error)
}

type persistentVolumeClaimLifecycleAdapter struct {
	lifecycle PersistentVolumeClaimLifecycle
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
	adapter := &persistentVolumeClaimLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.PersistentVolumeClaim) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
