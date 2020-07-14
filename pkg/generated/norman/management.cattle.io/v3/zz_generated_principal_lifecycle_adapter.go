package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type PrincipalLifecycle interface {
	Create(obj *v3.Principal) (runtime.Object, error)
	Remove(obj *v3.Principal) (runtime.Object, error)
	Updated(obj *v3.Principal) (runtime.Object, error)
}

type principalLifecycleAdapter struct {
	lifecycle PrincipalLifecycle
}

func (w *principalLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *principalLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *principalLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *principalLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *principalLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.Principal))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPrincipalLifecycleAdapter(name string, clusterScoped bool, client PrincipalInterface, l PrincipalLifecycle) PrincipalHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PrincipalGroupVersionResource)
	}
	adapter := &principalLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.Principal) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
