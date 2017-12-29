package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceLifecycle interface {
	Create(obj *v1.Service) (*v1.Service, error)
	Remove(obj *v1.Service) (*v1.Service, error)
	Updated(obj *v1.Service) (*v1.Service, error)
}

type serviceLifecycleAdapter struct {
	lifecycle ServiceLifecycle
}

func (w *serviceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Service))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Service))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Service))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceLifecycleAdapter(name string, client ServiceInterface, l ServiceLifecycle) ServiceHandlerFunc {
	adapter := &serviceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1.Service) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
