package v1beta2

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
)

type DaemonSetLifecycle interface {
	Create(obj *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error)
	Remove(obj *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error)
	Updated(obj *v1beta2.DaemonSet) (*v1beta2.DaemonSet, error)
}

type daemonSetLifecycleAdapter struct {
	lifecycle DaemonSetLifecycle
}

func (w *daemonSetLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1beta2.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *daemonSetLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1beta2.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *daemonSetLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1beta2.DaemonSet))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDaemonSetLifecycleAdapter(name string, clusterScoped bool, client DaemonSetInterface, l DaemonSetLifecycle) DaemonSetHandlerFunc {
	adapter := &daemonSetLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1beta2.DaemonSet) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
