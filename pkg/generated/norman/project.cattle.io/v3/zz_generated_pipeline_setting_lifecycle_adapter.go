package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type PipelineSettingLifecycle interface {
	Create(obj *v3.PipelineSetting) (runtime.Object, error)
	Remove(obj *v3.PipelineSetting) (runtime.Object, error)
	Updated(obj *v3.PipelineSetting) (runtime.Object, error)
}

type pipelineSettingLifecycleAdapter struct {
	lifecycle PipelineSettingLifecycle
}

func (w *pipelineSettingLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *pipelineSettingLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *pipelineSettingLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineSettingLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *pipelineSettingLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.PipelineSetting))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPipelineSettingLifecycleAdapter(name string, clusterScoped bool, client PipelineSettingInterface, l PipelineSettingLifecycle) PipelineSettingHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PipelineSettingGroupVersionResource)
	}
	adapter := &pipelineSettingLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.PipelineSetting) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
