package v1

import (
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type AlertmanagerLifecycle interface {
	Create(obj *v1.Alertmanager) (runtime.Object, error)
	Remove(obj *v1.Alertmanager) (runtime.Object, error)
	Updated(obj *v1.Alertmanager) (runtime.Object, error)
}

type alertmanagerLifecycleAdapter struct {
	lifecycle AlertmanagerLifecycle
}

func (w *alertmanagerLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *alertmanagerLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *alertmanagerLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Alertmanager))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *alertmanagerLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Alertmanager))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *alertmanagerLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Alertmanager))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewAlertmanagerLifecycleAdapter(name string, clusterScoped bool, client AlertmanagerInterface, l AlertmanagerLifecycle) AlertmanagerHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(AlertmanagerGroupVersionResource)
	}
	adapter := &alertmanagerLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Alertmanager) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
