package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type TokenLifecycle interface {
	Create(obj *Token) (*Token, error)
	Remove(obj *Token) (*Token, error)
	Updated(obj *Token) (*Token, error)
}

type tokenLifecycleAdapter struct {
	lifecycle TokenLifecycle
}

func (w *tokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Token))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *tokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Token))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *tokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Token))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewTokenLifecycleAdapter(name string, client TokenInterface, l TokenLifecycle) TokenHandlerFunc {
	adapter := &tokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *Token) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
