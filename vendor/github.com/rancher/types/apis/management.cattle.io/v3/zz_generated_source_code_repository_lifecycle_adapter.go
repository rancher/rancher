package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type SourceCodeRepositoryLifecycle interface {
	Create(obj *SourceCodeRepository) (*SourceCodeRepository, error)
	Remove(obj *SourceCodeRepository) (*SourceCodeRepository, error)
	Updated(obj *SourceCodeRepository) (*SourceCodeRepository, error)
}

type sourceCodeRepositoryLifecycleAdapter struct {
	lifecycle SourceCodeRepositoryLifecycle
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
	adapter := &sourceCodeRepositoryLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *SourceCodeRepository) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
