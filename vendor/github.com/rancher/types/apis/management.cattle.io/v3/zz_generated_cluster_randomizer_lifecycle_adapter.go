package v3

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterRandomizerLifecycle interface {
	Create(obj *ClusterRandomizer) (runtime.Object, error)
	Remove(obj *ClusterRandomizer) (runtime.Object, error)
	Updated(obj *ClusterRandomizer) (runtime.Object, error)
}

type clusterRandomizerLifecycleAdapter struct {
	lifecycle ClusterRandomizerLifecycle
}

func (w *clusterRandomizerLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterRandomizerLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterRandomizerLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterRandomizer))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRandomizerLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterRandomizer))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterRandomizerLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterRandomizer))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterRandomizerLifecycleAdapter(name string, clusterScoped bool, client ClusterRandomizerInterface, l ClusterRandomizerLifecycle) ClusterRandomizerHandlerFunc {
	adapter := &clusterRandomizerLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterRandomizer) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
