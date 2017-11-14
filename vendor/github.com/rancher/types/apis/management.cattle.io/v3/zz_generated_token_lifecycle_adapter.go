package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type TokenLifecycle interface {
	Create(obj *Token) error
	Remove(obj *Token) error
	Updated(obj *Token) error
}

type tokenLifecycleAdapter struct {
	lifecycle TokenLifecycle
}

func (w *tokenLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*Token))
}

func (w *tokenLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*Token))
}

func (w *tokenLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*Token))
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
