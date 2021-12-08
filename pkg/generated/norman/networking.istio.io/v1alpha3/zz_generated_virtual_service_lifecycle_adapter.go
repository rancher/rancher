package v1alpha3

import (
	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type VirtualServiceLifecycle interface {
	Create(obj *v1alpha3.VirtualService) (runtime.Object, error)
	Remove(obj *v1alpha3.VirtualService) (runtime.Object, error)
	Updated(obj *v1alpha3.VirtualService) (runtime.Object, error)
}

type virtualServiceLifecycleAdapter struct {
	lifecycle VirtualServiceLifecycle
}

func (w *virtualServiceLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *virtualServiceLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *virtualServiceLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1alpha3.VirtualService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *virtualServiceLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1alpha3.VirtualService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *virtualServiceLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1alpha3.VirtualService))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewVirtualServiceLifecycleAdapter(name string, clusterScoped bool, client VirtualServiceInterface, l VirtualServiceLifecycle) VirtualServiceHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(VirtualServiceGroupVersionResource)
	}
	adapter := &virtualServiceLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1alpha3.VirtualService) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
