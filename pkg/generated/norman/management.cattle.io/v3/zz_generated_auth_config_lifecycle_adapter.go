package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type AuthConfigLifecycle interface {
	Create(obj *v3.AuthConfig) (runtime.Object, error)
	Remove(obj *v3.AuthConfig) (runtime.Object, error)
	Updated(obj *v3.AuthConfig) (runtime.Object, error)
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
	o, err := w.lifecycle.Create(obj.(*v3.AuthConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.AuthConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *authConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.AuthConfig))
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
	return func(key string, obj *v3.AuthConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
