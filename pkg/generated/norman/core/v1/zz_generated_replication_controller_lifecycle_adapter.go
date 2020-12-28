package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ReplicationControllerLifecycle interface {
	Create(obj *v1.ReplicationController) (runtime.Object, error)
	Remove(obj *v1.ReplicationController) (runtime.Object, error)
	Updated(obj *v1.ReplicationController) (runtime.Object, error)
}

type replicationControllerLifecycleAdapter struct {
	lifecycle ReplicationControllerLifecycle
}

func (w *replicationControllerLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *replicationControllerLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *replicationControllerLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ReplicationController))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicationControllerLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ReplicationController))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *replicationControllerLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ReplicationController))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewReplicationControllerLifecycleAdapter(name string, clusterScoped bool, client ReplicationControllerInterface, l ReplicationControllerLifecycle) ReplicationControllerHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ReplicationControllerGroupVersionResource)
	}
	adapter := &replicationControllerLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ReplicationController) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
