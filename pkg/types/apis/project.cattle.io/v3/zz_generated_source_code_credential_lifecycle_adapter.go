package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeCredentialLifecycle interface {
	Create(obj *SourceCodeCredential) (runtime.Object, error)
	Remove(obj *SourceCodeCredential) (runtime.Object, error)
	Updated(obj *SourceCodeCredential) (runtime.Object, error)
}

type sourceCodeCredentialLifecycleAdapter struct {
	lifecycle SourceCodeCredentialLifecycle
}

func (w *sourceCodeCredentialLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *sourceCodeCredentialLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *sourceCodeCredentialLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*SourceCodeCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeCredentialLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*SourceCodeCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeCredentialLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*SourceCodeCredential))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSourceCodeCredentialLifecycleAdapter(name string, clusterScoped bool, client SourceCodeCredentialInterface, l SourceCodeCredentialLifecycle) SourceCodeCredentialHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(SourceCodeCredentialGroupVersionResource)
	}
	adapter := &sourceCodeCredentialLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SourceCodeCredential) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
