package v3public

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthProviderLifecycle interface {
	Create(obj *AuthProvider) (*AuthProvider, error)
	Remove(obj *AuthProvider) (*AuthProvider, error)
	Updated(obj *AuthProvider) (*AuthProvider, error)
}

type authProviderLifecycleAdapter struct {
	lifecycle AuthProviderLifecycle
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
	adapter := &authProviderLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AuthProvider) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
