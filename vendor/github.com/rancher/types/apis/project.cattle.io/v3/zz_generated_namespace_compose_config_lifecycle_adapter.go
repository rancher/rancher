package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type NamespaceComposeConfigLifecycle interface {
	Create(obj *NamespaceComposeConfig) (*NamespaceComposeConfig, error)
	Remove(obj *NamespaceComposeConfig) (*NamespaceComposeConfig, error)
	Updated(obj *NamespaceComposeConfig) (*NamespaceComposeConfig, error)
}

type namespaceComposeConfigLifecycleAdapter struct {
	lifecycle NamespaceComposeConfigLifecycle
}

func (w *namespaceComposeConfigLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*NamespaceComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespaceComposeConfigLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*NamespaceComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *namespaceComposeConfigLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*NamespaceComposeConfig))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewNamespaceComposeConfigLifecycleAdapter(name string, clusterScoped bool, client NamespaceComposeConfigInterface, l NamespaceComposeConfigLifecycle) NamespaceComposeConfigHandlerFunc {
	adapter := &namespaceComposeConfigLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *NamespaceComposeConfig) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
