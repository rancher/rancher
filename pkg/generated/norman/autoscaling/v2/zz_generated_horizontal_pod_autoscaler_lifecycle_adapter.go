package v2

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/runtime"
)

type HorizontalPodAutoscalerLifecycle interface {
	Create(obj *v2.HorizontalPodAutoscaler) (runtime.Object, error)
	Remove(obj *v2.HorizontalPodAutoscaler) (runtime.Object, error)
	Updated(obj *v2.HorizontalPodAutoscaler) (runtime.Object, error)
}

type horizontalPodAutoscalerLifecycleAdapter struct {
	lifecycle HorizontalPodAutoscalerLifecycle
}

func (w *horizontalPodAutoscalerLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *horizontalPodAutoscalerLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *horizontalPodAutoscalerLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v2.HorizontalPodAutoscaler))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *horizontalPodAutoscalerLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v2.HorizontalPodAutoscaler))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *horizontalPodAutoscalerLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v2.HorizontalPodAutoscaler))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewHorizontalPodAutoscalerLifecycleAdapter(name string, clusterScoped bool, client HorizontalPodAutoscalerInterface, l HorizontalPodAutoscalerLifecycle) HorizontalPodAutoscalerHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(HorizontalPodAutoscalerGroupVersionResource)
	}
	adapter := &horizontalPodAutoscalerLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v2.HorizontalPodAutoscaler) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
