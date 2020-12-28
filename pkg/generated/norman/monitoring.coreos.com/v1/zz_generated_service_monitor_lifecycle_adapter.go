package v1

import (
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceMonitorLifecycle interface {
	Create(obj *v1.ServiceMonitor) (runtime.Object, error)
	Remove(obj *v1.ServiceMonitor) (runtime.Object, error)
	Updated(obj *v1.ServiceMonitor) (runtime.Object, error)
}

type serviceMonitorLifecycleAdapter struct {
	lifecycle ServiceMonitorLifecycle
}

func (w *serviceMonitorLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *serviceMonitorLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *serviceMonitorLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.ServiceMonitor))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceMonitorLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.ServiceMonitor))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *serviceMonitorLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.ServiceMonitor))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewServiceMonitorLifecycleAdapter(name string, clusterScoped bool, client ServiceMonitorInterface, l ServiceMonitorLifecycle) ServiceMonitorHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ServiceMonitorGroupVersionResource)
	}
	adapter := &serviceMonitorLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.ServiceMonitor) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
