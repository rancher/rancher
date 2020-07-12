package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type TokenLifecycle interface {
	Create(obj *Token) (runtime.Object, error)
	Remove(obj *Token) (runtime.Object, error)
	Updated(obj *Token) (runtime.Object, error)
}

type tokenLifecycleAdapter struct {
	lifecycle TokenLifecycle
}

func (w *tokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *tokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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

func NewTokenLifecycleAdapter(name string, clusterScoped bool, client TokenInterface, l TokenLifecycle) TokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(TokenGroupVersionResource)
	}
	adapter := &tokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Token) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
