package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type RancherUserNotificationLifecycle interface {
	Create(obj *v3.RancherUserNotification) (runtime.Object, error)
	Remove(obj *v3.RancherUserNotification) (runtime.Object, error)
	Updated(obj *v3.RancherUserNotification) (runtime.Object, error)
}

type rancherUserNotificationLifecycleAdapter struct {
	lifecycle RancherUserNotificationLifecycle
}

func (w *rancherUserNotificationLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *rancherUserNotificationLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *rancherUserNotificationLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.RancherUserNotification))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rancherUserNotificationLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.RancherUserNotification))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *rancherUserNotificationLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.RancherUserNotification))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewRancherUserNotificationLifecycleAdapter(name string, clusterScoped bool, client RancherUserNotificationInterface, l RancherUserNotificationLifecycle) RancherUserNotificationHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(RancherUserNotificationGroupVersionResource)
	}
	adapter := &rancherUserNotificationLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.RancherUserNotification) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
