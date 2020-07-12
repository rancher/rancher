package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type MonitorMetricLifecycle interface {
	Create(obj *MonitorMetric) (runtime.Object, error)
	Remove(obj *MonitorMetric) (runtime.Object, error)
	Updated(obj *MonitorMetric) (runtime.Object, error)
}

type monitorMetricLifecycleAdapter struct {
	lifecycle MonitorMetricLifecycle
}

func (w *monitorMetricLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *monitorMetricLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *monitorMetricLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*MonitorMetric))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *monitorMetricLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*MonitorMetric))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *monitorMetricLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*MonitorMetric))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewMonitorMetricLifecycleAdapter(name string, clusterScoped bool, client MonitorMetricInterface, l MonitorMetricLifecycle) MonitorMetricHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(MonitorMetricGroupVersionResource)
	}
	adapter := &monitorMetricLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *MonitorMetric) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
