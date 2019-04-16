package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EndpointsLifecycle interface {
	Create(obj *v1.Endpoints) (*v1.Endpoints, error)
	Remove(obj *v1.Endpoints) (*v1.Endpoints, error)
	Updated(obj *v1.Endpoints) (*v1.Endpoints, error)
}

type endpointsLifecycleAdapter struct {
	lifecycle EndpointsLifecycle
}

func (w *endpointsLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *endpointsLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *endpointsLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Endpoints))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewEndpointsLifecycleAdapter(name string, clusterScoped bool, client EndpointsInterface, l EndpointsLifecycle) EndpointsHandlerFunc {
	adapter := &endpointsLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Endpoints) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
