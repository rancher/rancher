package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type CertificateLifecycle interface {
	Create(obj *v3.Certificate) (runtime.Object, error)
	Remove(obj *v3.Certificate) (runtime.Object, error)
	Updated(obj *v3.Certificate) (runtime.Object, error)
}

type certificateLifecycleAdapter struct {
	lifecycle CertificateLifecycle
}

func (w *certificateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *certificateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *certificateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *certificateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *certificateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCertificateLifecycleAdapter(name string, clusterScoped bool, client CertificateInterface, l CertificateLifecycle) CertificateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CertificateGroupVersionResource)
	}
	adapter := &certificateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Certificate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
