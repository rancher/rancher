package v1beta2

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
)

type ReplicaSetLifecycle interface {
	Create(obj *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error)
	Remove(obj *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error)
	Updated(obj *v1beta2.ReplicaSet) (*v1beta2.ReplicaSet, error)
}

type replicaSetLifecycleAdapter struct {
	lifecycle ReplicaSetLifecycle
}

func (w *replicaSetLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1beta2.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicaSetLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1beta2.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicaSetLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1beta2.ReplicaSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewReplicaSetLifecycleAdapter(name string, clusterScoped bool, client ReplicaSetInterface, l ReplicaSetLifecycle) ReplicaSetHandlerFunc {
	adapter := &replicaSetLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta2.ReplicaSet) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
