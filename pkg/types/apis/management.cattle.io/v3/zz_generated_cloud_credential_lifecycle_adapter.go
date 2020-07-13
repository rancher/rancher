package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type CloudCredentialLifecycle interface {
	Create(obj *CloudCredential) (runtime.Object, error)
	Remove(obj *CloudCredential) (runtime.Object, error)
	Updated(obj *CloudCredential) (runtime.Object, error)
}

type cloudCredentialLifecycleAdapter struct {
	lifecycle CloudCredentialLifecycle
}

func (w *cloudCredentialLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *cloudCredentialLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
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
	if clusterScoped {
		resource.PutClusterScoped(CloudCredentialGroupVersionResource)
	}
	adapter := &cloudCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *CloudCredential) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
