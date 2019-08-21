package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type RKEK8sServiceOptionLifecycle interface {
	Create(obj *RKEK8sServiceOption) (runtime.Object, error)
	Remove(obj *RKEK8sServiceOption) (runtime.Object, error)
	Updated(obj *RKEK8sServiceOption) (runtime.Object, error)
}

type rkeK8sServiceOptionLifecycleAdapter struct {
	lifecycle RKEK8sServiceOptionLifecycle
}

func (w *rkeK8sServiceOptionLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *rkeK8sServiceOptionLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *rkeK8sServiceOptionLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*RKEK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sServiceOptionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*RKEK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sServiceOptionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*RKEK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRKEK8sServiceOptionLifecycleAdapter(name string, clusterScoped bool, client RKEK8sServiceOptionInterface, l RKEK8sServiceOptionLifecycle) RKEK8sServiceOptionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RKEK8sServiceOptionGroupVersionResource)
	}
	adapter := &rkeK8sServiceOptionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *RKEK8sServiceOption) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
