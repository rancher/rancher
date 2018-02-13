package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterPipelineLifecycle interface {
	Create(obj *ClusterPipeline) (*ClusterPipeline, error)
	Remove(obj *ClusterPipeline) (*ClusterPipeline, error)
	Updated(obj *ClusterPipeline) (*ClusterPipeline, error)
}

type clusterPipelineLifecycleAdapter struct {
	lifecycle ClusterPipelineLifecycle
}

func (w *clusterPipelineLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterPipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterPipelineLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterPipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterPipelineLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterPipeline))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterPipelineLifecycleAdapter(name string, clusterScoped bool, client ClusterPipelineInterface, l ClusterPipelineLifecycle) ClusterPipelineHandlerFunc {
	adapter := &clusterPipelineLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterPipeline) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
