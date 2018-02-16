package v1beta1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type IngressLifecycle interface {
	Create(obj *v1beta1.Ingress) (*v1beta1.Ingress, error)
	Remove(obj *v1beta1.Ingress) (*v1beta1.Ingress, error)
	Updated(obj *v1beta1.Ingress) (*v1beta1.Ingress, error)
}

type ingressLifecycleAdapter struct {
	lifecycle IngressLifecycle
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
	adapter := &ingressLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta1.Ingress) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
