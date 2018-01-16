package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type CertificateLifecycle interface {
	Create(obj *Certificate) (*Certificate, error)
	Remove(obj *Certificate) (*Certificate, error)
	Updated(obj *Certificate) (*Certificate, error)
}

type certificateLifecycleAdapter struct {
	lifecycle CertificateLifecycle
}

func (w *certificateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *certificateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *certificateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Certificate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCertificateLifecycleAdapter(name string, clusterScoped bool, client CertificateInterface, l CertificateLifecycle) CertificateHandlerFunc {
	adapter := &certificateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Certificate) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
