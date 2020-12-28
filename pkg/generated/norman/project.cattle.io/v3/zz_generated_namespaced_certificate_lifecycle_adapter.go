package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedCertificateLifecycle interface {
	Create(obj *v3.NamespacedCertificate) (runtime.Object, error)
	Remove(obj *v3.NamespacedCertificate) (runtime.Object, error)
	Updated(obj *v3.NamespacedCertificate) (runtime.Object, error)
}

type namespacedCertificateLifecycleAdapter struct {
	lifecycle NamespacedCertificateLifecycle
}

func (w *namespacedCertificateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespacedCertificateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespacedCertificateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.NamespacedCertificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedCertificateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.NamespacedCertificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedCertificateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.NamespacedCertificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedCertificateLifecycleAdapter(name string, clusterScoped bool, client NamespacedCertificateInterface, l NamespacedCertificateLifecycle) NamespacedCertificateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespacedCertificateGroupVersionResource)
	}
	adapter := &namespacedCertificateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.NamespacedCertificate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
