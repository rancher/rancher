package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type AppRevisionLifecycle interface {
	Create(obj *v3.AppRevision) (runtime.Object, error)
	Remove(obj *v3.AppRevision) (runtime.Object, error)
	Updated(obj *v3.AppRevision) (runtime.Object, error)
}

type appRevisionLifecycleAdapter struct {
	lifecycle AppRevisionLifecycle
}

func (w *appRevisionLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *appRevisionLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *appRevisionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appRevisionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *appRevisionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.AppRevision))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAppRevisionLifecycleAdapter(name string, clusterScoped bool, client AppRevisionInterface, l AppRevisionLifecycle) AppRevisionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(AppRevisionGroupVersionResource)
	}
	adapter := &appRevisionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.AppRevision) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
