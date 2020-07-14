package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type FeatureLifecycle interface {
	Create(obj *v3.Feature) (runtime.Object, error)
	Remove(obj *v3.Feature) (runtime.Object, error)
	Updated(obj *v3.Feature) (runtime.Object, error)
}

type featureLifecycleAdapter struct {
	lifecycle FeatureLifecycle
}

func (w *featureLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *featureLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *featureLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Feature))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *featureLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Feature))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *featureLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Feature))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewFeatureLifecycleAdapter(name string, clusterScoped bool, client FeatureInterface, l FeatureLifecycle) FeatureHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(FeatureGroupVersionResource)
	}
	adapter := &featureLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Feature) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
