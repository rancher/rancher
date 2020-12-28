package v1

import (
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type PrometheusLifecycle interface {
	Create(obj *v1.Prometheus) (runtime.Object, error)
	Remove(obj *v1.Prometheus) (runtime.Object, error)
	Updated(obj *v1.Prometheus) (runtime.Object, error)
}

type prometheusLifecycleAdapter struct {
	lifecycle PrometheusLifecycle
}

func (w *prometheusLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *prometheusLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *prometheusLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.Prometheus))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *prometheusLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.Prometheus))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *prometheusLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.Prometheus))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPrometheusLifecycleAdapter(name string, clusterScoped bool, client PrometheusInterface, l PrometheusLifecycle) PrometheusHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PrometheusGroupVersionResource)
	}
	adapter := &prometheusLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.Prometheus) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
