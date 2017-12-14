package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectLifecycle interface {
	Create(obj *Project) (*Project, error)
	Remove(obj *Project) (*Project, error)
	Updated(obj *Project) (*Project, error)
}

type projectLifecycleAdapter struct {
	lifecycle ProjectLifecycle
}

func (w *projectLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Project))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Project))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Project))
	if o == nil {
		return nil, err
	}
	return o, err
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
