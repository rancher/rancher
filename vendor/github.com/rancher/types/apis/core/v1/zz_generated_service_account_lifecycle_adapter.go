package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceAccountLifecycle interface {
	Create(obj *v1.ServiceAccount) (*v1.ServiceAccount, error)
	Remove(obj *v1.ServiceAccount) (*v1.ServiceAccount, error)
	Updated(obj *v1.ServiceAccount) (*v1.ServiceAccount, error)
}

type serviceAccountLifecycleAdapter struct {
	lifecycle ServiceAccountLifecycle
}

func (w *serviceAccountLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ServiceAccount))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceAccountLifecycleAdapter(name string, clusterScoped bool, client ServiceAccountInterface, l ServiceAccountLifecycle) ServiceAccountHandlerFunc {
	adapter := &serviceAccountLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ServiceAccount) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
