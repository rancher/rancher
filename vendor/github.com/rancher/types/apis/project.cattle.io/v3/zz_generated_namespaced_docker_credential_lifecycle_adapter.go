package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespacedDockerCredentialLifecycle interface {
	Create(obj *NamespacedDockerCredential) (runtime.Object, error)
	Remove(obj *NamespacedDockerCredential) (runtime.Object, error)
	Updated(obj *NamespacedDockerCredential) (runtime.Object, error)
}

type namespacedDockerCredentialLifecycleAdapter struct {
	lifecycle NamespacedDockerCredentialLifecycle
}

func (w *namespacedDockerCredentialLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *namespacedDockerCredentialLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *namespacedDockerCredentialLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedDockerCredentialLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespacedDockerCredentialLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespacedDockerCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespacedDockerCredentialLifecycleAdapter(name string, clusterScoped bool, client NamespacedDockerCredentialInterface, l NamespacedDockerCredentialLifecycle) NamespacedDockerCredentialHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(NamespacedDockerCredentialGroupVersionResource)
	}
	adapter := &namespacedDockerCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NamespacedDockerCredential) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
