package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectLifecycle interface {
	Create(obj *v3.Project) (runtime.Object, error)
	Remove(obj *v3.Project) (runtime.Object, error)
	Updated(obj *v3.Project) (runtime.Object, error)
}

type projectLifecycleAdapter struct {
	lifecycle ProjectLifecycle
}

func (w *projectLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Project))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Project))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Project))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectLifecycleAdapter(name string, clusterScoped bool, client ProjectInterface, l ProjectLifecycle) ProjectHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectGroupVersionResource)
	}
	adapter := &projectLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Project) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
