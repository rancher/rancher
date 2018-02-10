package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type AppLifecycle interface {
	Create(obj *App) (*App, error)
	Remove(obj *App) (*App, error)
	Updated(obj *App) (*App, error)
}

type appLifecycleAdapter struct {
	lifecycle AppLifecycle
}

func (w *appLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*App))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*App))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*App))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAppLifecycleAdapter(name string, clusterScoped bool, client AppInterface, l AppLifecycle) AppHandlerFunc {
	adapter := &appLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *App) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
