package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceAccountTokenLifecycle interface {
	Create(obj *v3.ServiceAccountToken) (runtime.Object, error)
	Remove(obj *v3.ServiceAccountToken) (runtime.Object, error)
	Updated(obj *v3.ServiceAccountToken) (runtime.Object, error)
}

type serviceAccountTokenLifecycleAdapter struct {
	lifecycle ServiceAccountTokenLifecycle
}

func (w *serviceAccountTokenLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *serviceAccountTokenLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *serviceAccountTokenLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountTokenLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceAccountTokenLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.ServiceAccountToken))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceAccountTokenLifecycleAdapter(name string, clusterScoped bool, client ServiceAccountTokenInterface, l ServiceAccountTokenLifecycle) ServiceAccountTokenHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ServiceAccountTokenGroupVersionResource)
	}
	adapter := &serviceAccountTokenLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.ServiceAccountToken) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
