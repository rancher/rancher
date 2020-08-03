package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ReplicaSetLifecycle interface {
	Create(obj *v1.ReplicaSet) (runtime.Object, error)
	Remove(obj *v1.ReplicaSet) (runtime.Object, error)
	Updated(obj *v1.ReplicaSet) (runtime.Object, error)
}

type replicaSetLifecycleAdapter struct {
	lifecycle ReplicaSetLifecycle
}

func (w *replicaSetLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *replicaSetLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *replicaSetLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicaSetLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicaSetLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewReplicaSetLifecycleAdapter(name string, clusterScoped bool, client ReplicaSetInterface, l ReplicaSetLifecycle) ReplicaSetHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ReplicaSetGroupVersionResource)
	}
	adapter := &replicaSetLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ReplicaSet) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
