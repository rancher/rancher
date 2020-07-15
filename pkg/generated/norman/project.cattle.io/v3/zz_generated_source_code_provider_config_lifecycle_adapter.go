package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeProviderConfigLifecycle interface {
	Create(obj *v3.SourceCodeProviderConfig) (runtime.Object, error)
	Remove(obj *v3.SourceCodeProviderConfig) (runtime.Object, error)
	Updated(obj *v3.SourceCodeProviderConfig) (runtime.Object, error)
}

type sourceCodeProviderConfigLifecycleAdapter struct {
	lifecycle SourceCodeProviderConfigLifecycle
}

func (w *sourceCodeProviderConfigLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *sourceCodeProviderConfigLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *sourceCodeProviderConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.SourceCodeProviderConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeProviderConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.SourceCodeProviderConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeProviderConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.SourceCodeProviderConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSourceCodeProviderConfigLifecycleAdapter(name string, clusterScoped bool, client SourceCodeProviderConfigInterface, l SourceCodeProviderConfigLifecycle) SourceCodeProviderConfigHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(SourceCodeProviderConfigGroupVersionResource)
	}
	adapter := &sourceCodeProviderConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.SourceCodeProviderConfig) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
