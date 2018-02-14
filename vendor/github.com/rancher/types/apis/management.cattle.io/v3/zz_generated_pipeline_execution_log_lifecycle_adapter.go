package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineExecutionLogLifecycle interface {
	Create(obj *PipelineExecutionLog) (*PipelineExecutionLog, error)
	Remove(obj *PipelineExecutionLog) (*PipelineExecutionLog, error)
	Updated(obj *PipelineExecutionLog) (*PipelineExecutionLog, error)
}

type pipelineExecutionLogLifecycleAdapter struct {
	lifecycle PipelineExecutionLogLifecycle
}

func (w *pipelineExecutionLogLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*PipelineExecutionLog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineExecutionLogLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*PipelineExecutionLog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineExecutionLogLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*PipelineExecutionLog))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPipelineExecutionLogLifecycleAdapter(name string, clusterScoped bool, client PipelineExecutionLogInterface, l PipelineExecutionLogLifecycle) PipelineExecutionLogHandlerFunc {
	adapter := &pipelineExecutionLogLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *PipelineExecutionLog) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
