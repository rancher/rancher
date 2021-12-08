package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

type APIServiceLifecycle interface {
	Create(obj *v1.APIService) (runtime.Object, error)
	Remove(obj *v1.APIService) (runtime.Object, error)
	Updated(obj *v1.APIService) (runtime.Object, error)
}

type apiServiceLifecycleAdapter struct {
	lifecycle APIServiceLifecycle
}

func (w *apiServiceLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *apiServiceLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *apiServiceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.APIService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *apiServiceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.APIService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *apiServiceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.APIService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAPIServiceLifecycleAdapter(name string, clusterScoped bool, client APIServiceInterface, l APIServiceLifecycle) APIServiceHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(APIServiceGroupVersionResource)
	}
	adapter := &apiServiceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.APIService) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
