package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceAccountTokenLifecycle interface {
	Create(obj *ServiceAccountToken) (*ServiceAccountToken, error)
	Remove(obj *ServiceAccountToken) (*ServiceAccountToken, error)
	Updated(obj *ServiceAccountToken) (*ServiceAccountToken, error)
}

type serviceAccountTokenLifecycleAdapter struct {
	lifecycle ServiceAccountTokenLifecycle
}

func (w *serviceAccountTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceAccountTokenLifecycleAdapter(name string, clusterScoped bool, client ServiceAccountTokenInterface, l ServiceAccountTokenLifecycle) ServiceAccountTokenHandlerFunc {
	adapter := &serviceAccountTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ServiceAccountToken) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
