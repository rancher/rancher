package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeProviderLifecycle interface {
	Create(obj *SourceCodeProvider) (*SourceCodeProvider, error)
	Remove(obj *SourceCodeProvider) (*SourceCodeProvider, error)
	Updated(obj *SourceCodeProvider) (*SourceCodeProvider, error)
}

type sourceCodeProviderLifecycleAdapter struct {
	lifecycle SourceCodeProviderLifecycle
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
	adapter := &sourceCodeProviderLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SourceCodeProvider) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
