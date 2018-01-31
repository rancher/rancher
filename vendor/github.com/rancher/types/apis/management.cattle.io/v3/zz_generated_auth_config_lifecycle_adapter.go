package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthConfigLifecycle interface {
	Create(obj *AuthConfig) (*AuthConfig, error)
	Remove(obj *AuthConfig) (*AuthConfig, error)
	Updated(obj *AuthConfig) (*AuthConfig, error)
}

type authConfigLifecycleAdapter struct {
	lifecycle AuthConfigLifecycle
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
	adapter := &authConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *AuthConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
