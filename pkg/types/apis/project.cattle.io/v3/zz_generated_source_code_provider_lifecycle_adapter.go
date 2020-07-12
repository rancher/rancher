package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeProviderLifecycle interface {
	Create(obj *SourceCodeProvider) (runtime.Object, error)
	Remove(obj *SourceCodeProvider) (runtime.Object, error)
	Updated(obj *SourceCodeProvider) (runtime.Object, error)
}

type sourceCodeProviderLifecycleAdapter struct {
	lifecycle SourceCodeProviderLifecycle
}

func (w *sourceCodeProviderLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *sourceCodeProviderLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *sourceCodeProviderLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*SourceCodeProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeProviderLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*SourceCodeProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeProviderLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*SourceCodeProvider))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSourceCodeProviderLifecycleAdapter(name string, clusterScoped bool, client SourceCodeProviderInterface, l SourceCodeProviderLifecycle) SourceCodeProviderHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(SourceCodeProviderGroupVersionResource)
	}
	adapter := &sourceCodeProviderLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SourceCodeProvider) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
