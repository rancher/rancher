package v1beta1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IngressLifecycle interface {
	Create(obj *v1beta1.Ingress) (runtime.Object, error)
	Remove(obj *v1beta1.Ingress) (runtime.Object, error)
	Updated(obj *v1beta1.Ingress) (runtime.Object, error)
}

type ingressLifecycleAdapter struct {
	lifecycle IngressLifecycle
}

func (w *ingressLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *ingressLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *ingressLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1beta1.Ingress))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ingressLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1beta1.Ingress))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *ingressLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1beta1.Ingress))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewIngressLifecycleAdapter(name string, clusterScoped bool, client IngressInterface, l IngressLifecycle) IngressHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(IngressGroupVersionResource)
	}
	adapter := &ingressLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta1.Ingress) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
