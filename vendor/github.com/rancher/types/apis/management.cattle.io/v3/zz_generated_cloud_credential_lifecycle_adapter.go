package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type CloudCredentialLifecycle interface {
	Create(obj *CloudCredential) (*CloudCredential, error)
	Remove(obj *CloudCredential) (*CloudCredential, error)
	Updated(obj *CloudCredential) (*CloudCredential, error)
}

type cloudCredentialLifecycleAdapter struct {
	lifecycle CloudCredentialLifecycle
}

func (w *cloudCredentialLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*CloudCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cloudCredentialLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*CloudCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cloudCredentialLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*CloudCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCloudCredentialLifecycleAdapter(name string, clusterScoped bool, client CloudCredentialInterface, l CloudCredentialLifecycle) CloudCredentialHandlerFunc {
	adapter := &cloudCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CloudCredential) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
