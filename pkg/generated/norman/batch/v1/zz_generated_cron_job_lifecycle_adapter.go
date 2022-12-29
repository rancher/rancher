package v1

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type CronJobLifecycle interface {
	Create(obj *v1.CronJob) (runtime.Object, error)
	Remove(obj *v1.CronJob) (runtime.Object, error)
	Updated(obj *v1.CronJob) (runtime.Object, error)
}

type cronJobLifecycleAdapter struct {
	lifecycle CronJobLifecycle
}

func (w *cronJobLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *cronJobLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *cronJobLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1.CronJob))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cronJobLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1.CronJob))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *cronJobLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1.CronJob))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewCronJobLifecycleAdapter(name string, clusterScoped bool, client CronJobInterface, l CronJobLifecycle) CronJobHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(CronJobGroupVersionResource)
	}
	adapter := &cronJobLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1.CronJob) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
