package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineLifecycle interface {
	Create(obj *v3.Pipeline) (runtime.Object, error)
	Remove(obj *v3.Pipeline) (runtime.Object, error)
	Updated(obj *v3.Pipeline) (runtime.Object, error)
}

type pipelineLifecycleAdapter struct {
	lifecycle PipelineLifecycle
}

func (w *pipelineLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *pipelineLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *pipelineLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Pipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPipelineLifecycleAdapter(name string, clusterScoped bool, client PipelineInterface, l PipelineLifecycle) PipelineHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PipelineGroupVersionResource)
	}
	adapter := &pipelineLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Pipeline) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
