package v3public

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthTokenLifecycle interface {
	Create(obj *AuthToken) (runtime.Object, error)
	Remove(obj *AuthToken) (runtime.Object, error)
	Updated(obj *AuthToken) (runtime.Object, error)
}

type authTokenLifecycleAdapter struct {
	lifecycle AuthTokenLifecycle
}

func (w *authTokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *authTokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *authTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*AuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*AuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*AuthToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAuthTokenLifecycleAdapter(name string, clusterScoped bool, client AuthTokenInterface, l AuthTokenLifecycle) AuthTokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(AuthTokenGroupVersionResource)
	}
	adapter := &authTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AuthToken) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
