package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PrincipalLifecycle interface {
	Create(obj *Principal) (*Principal, error)
	Remove(obj *Principal) (*Principal, error)
	Updated(obj *Principal) (*Principal, error)
}

type principalLifecycleAdapter struct {
	lifecycle PrincipalLifecycle
}

func (w *principalLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *principalLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *principalLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPrincipalLifecycleAdapter(name string, client PrincipalInterface, l PrincipalLifecycle) PrincipalHandlerFunc {
	adapter := &principalLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Principal) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
