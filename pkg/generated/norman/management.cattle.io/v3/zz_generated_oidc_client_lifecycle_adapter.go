package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type OIDCClientLifecycle interface {
	Create(obj *v3.OIDCClient) (runtime.Object, error)
	Remove(obj *v3.OIDCClient) (runtime.Object, error)
	Updated(obj *v3.OIDCClient) (runtime.Object, error)
}

type oidcClientLifecycleAdapter struct {
	lifecycle OIDCClientLifecycle
}

func (w *oidcClientLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *oidcClientLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *oidcClientLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.OIDCClient))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *oidcClientLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.OIDCClient))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *oidcClientLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.OIDCClient))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewOIDCClientLifecycleAdapter(name string, clusterScoped bool, client OIDCClientInterface, l OIDCClientLifecycle) OIDCClientHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(OIDCClientGroupVersionResource)
	}
	adapter := &oidcClientLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.OIDCClient) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
