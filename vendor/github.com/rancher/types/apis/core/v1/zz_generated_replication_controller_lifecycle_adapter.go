package v1

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ReplicationControllerLifecycle interface {
	Create(obj *v1.ReplicationController) (*v1.ReplicationController, error)
	Remove(obj *v1.ReplicationController) (*v1.ReplicationController, error)
	Updated(obj *v1.ReplicationController) (*v1.ReplicationController, error)
}

type replicationControllerLifecycleAdapter struct {
	lifecycle ReplicationControllerLifecycle
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
	adapter := &replicationControllerLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ReplicationController) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
