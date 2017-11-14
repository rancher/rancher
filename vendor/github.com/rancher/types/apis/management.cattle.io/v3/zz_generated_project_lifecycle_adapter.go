package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectLifecycle interface {
	Create(obj *Project) error
	Remove(obj *Project) error
	Updated(obj *Project) error
}

type projectLifecycleAdapter struct {
	lifecycle ProjectLifecycle
}

func (w *projectLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Project))
}

func (w *projectLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Project))
}

func (w *projectLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Project))
}

func NewProjectLifecycleAdapter(name string, client ProjectInterface, l ProjectLifecycle) ProjectHandlerFunc {
	adapter := &projectLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Project) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
