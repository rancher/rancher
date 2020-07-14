package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type BasicAuthLifecycle interface {
	Create(obj *v3.BasicAuth) (runtime.Object, error)
	Remove(obj *v3.BasicAuth) (runtime.Object, error)
	Updated(obj *v3.BasicAuth) (runtime.Object, error)
}

type basicAuthLifecycleAdapter struct {
	lifecycle BasicAuthLifecycle
}

func (w *basicAuthLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *basicAuthLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *basicAuthLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.BasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *basicAuthLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.BasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *basicAuthLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.BasicAuth))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewBasicAuthLifecycleAdapter(name string, clusterScoped bool, client BasicAuthInterface, l BasicAuthLifecycle) BasicAuthHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(BasicAuthGroupVersionResource)
	}
	adapter := &basicAuthLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.BasicAuth) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
