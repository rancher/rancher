package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeRepositoryLifecycle interface {
	Create(obj *SourceCodeRepository) (runtime.Object, error)
	Remove(obj *SourceCodeRepository) (runtime.Object, error)
	Updated(obj *SourceCodeRepository) (runtime.Object, error)
}

type sourceCodeRepositoryLifecycleAdapter struct {
	lifecycle SourceCodeRepositoryLifecycle
}

func (w *sourceCodeRepositoryLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *sourceCodeRepositoryLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *sourceCodeRepositoryLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*SourceCodeRepository))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeRepositoryLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*SourceCodeRepository))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *sourceCodeRepositoryLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*SourceCodeRepository))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewSourceCodeRepositoryLifecycleAdapter(name string, clusterScoped bool, client SourceCodeRepositoryInterface, l SourceCodeRepositoryLifecycle) SourceCodeRepositoryHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(SourceCodeRepositoryGroupVersionResource)
	}
	adapter := &sourceCodeRepositoryLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SourceCodeRepository) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
