package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type RkeK8sServiceOptionLifecycle interface {
	Create(obj *v3.RkeK8sServiceOption) (runtime.Object, error)
	Remove(obj *v3.RkeK8sServiceOption) (runtime.Object, error)
	Updated(obj *v3.RkeK8sServiceOption) (runtime.Object, error)
}

type rkeK8sServiceOptionLifecycleAdapter struct {
	lifecycle RkeK8sServiceOptionLifecycle
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
	o, err := w.lifecycle.Create(obj.(*v3.RkeK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sServiceOptionLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.RkeK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rkeK8sServiceOptionLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.RkeK8sServiceOption))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRkeK8sServiceOptionLifecycleAdapter(name string, clusterScoped bool, client RkeK8sServiceOptionInterface, l RkeK8sServiceOptionLifecycle) RkeK8sServiceOptionHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RkeK8sServiceOptionGroupVersionResource)
	}
	adapter := &rkeK8sServiceOptionLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.RkeK8sServiceOption) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
