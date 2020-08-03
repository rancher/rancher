package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type TokenLifecycle interface {
	Create(obj *v3.Token) (runtime.Object, error)
	Remove(obj *v3.Token) (runtime.Object, error)
	Updated(obj *v3.Token) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.Token))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *tokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Token))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *tokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Token))
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
	return func(key string, obj *v3.Token) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
