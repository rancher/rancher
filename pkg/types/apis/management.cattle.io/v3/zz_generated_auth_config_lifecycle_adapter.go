package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthConfigLifecycle interface {
	Create(obj *AuthConfig) (runtime.Object, error)
	Remove(obj *AuthConfig) (runtime.Object, error)
	Updated(obj *AuthConfig) (runtime.Object, error)
}

type authConfigLifecycleAdapter struct {
	lifecycle AuthConfigLifecycle
}

func (w *authConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *authConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *authConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*AuthConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*AuthConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*AuthConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAuthConfigLifecycleAdapter(name string, clusterScoped bool, client AuthConfigInterface, l AuthConfigLifecycle) AuthConfigHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(AuthConfigGroupVersionResource)
	}
	adapter := &authConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AuthConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
