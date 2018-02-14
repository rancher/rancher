package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineLifecycle interface {
	Create(obj *Pipeline) (*Pipeline, error)
	Remove(obj *Pipeline) (*Pipeline, error)
	Updated(obj *Pipeline) (*Pipeline, error)
}

type pipelineLifecycleAdapter struct {
	lifecycle PipelineLifecycle
}

func (w *pipelineLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPipelineLifecycleAdapter(name string, clusterScoped bool, client PipelineInterface, l PipelineLifecycle) PipelineHandlerFunc {
	adapter := &pipelineLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *Pipeline) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
