package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type AppRevisionLifecycle interface {
	Create(obj *AppRevision) (*AppRevision, error)
	Remove(obj *AppRevision) (*AppRevision, error)
	Updated(obj *AppRevision) (*AppRevision, error)
}

type appRevisionLifecycleAdapter struct {
	lifecycle AppRevisionLifecycle
}

func (w *appRevisionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appRevisionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appRevisionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAppRevisionLifecycleAdapter(name string, clusterScoped bool, client AppRevisionInterface, l AppRevisionLifecycle) AppRevisionHandlerFunc {
	adapter := &appRevisionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AppRevision) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
