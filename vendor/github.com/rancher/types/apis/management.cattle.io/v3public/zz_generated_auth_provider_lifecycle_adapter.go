package v3public

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthProviderLifecycle interface {
	Create(obj *AuthProvider) (runtime.Object, error)
	Remove(obj *AuthProvider) (runtime.Object, error)
	Updated(obj *AuthProvider) (runtime.Object, error)
}

type authProviderLifecycleAdapter struct {
	lifecycle AuthProviderLifecycle
}

func (w *authProviderLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *authProviderLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *authProviderLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*AuthProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authProviderLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*AuthProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authProviderLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*AuthProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAuthProviderLifecycleAdapter(name string, clusterScoped bool, client AuthProviderInterface, l AuthProviderLifecycle) AuthProviderHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(AuthProviderGroupVersionResource)
	}
	adapter := &authProviderLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AuthProvider) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
