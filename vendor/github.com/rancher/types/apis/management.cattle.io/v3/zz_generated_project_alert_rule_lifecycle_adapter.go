package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ProjectAlertRuleLifecycle interface {
	Create(obj *ProjectAlertRule) (runtime.Object, error)
	Remove(obj *ProjectAlertRule) (runtime.Object, error)
	Updated(obj *ProjectAlertRule) (runtime.Object, error)
}

type projectAlertRuleLifecycleAdapter struct {
	lifecycle ProjectAlertRuleLifecycle
}

func (w *projectAlertRuleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *projectAlertRuleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *projectAlertRuleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ProjectAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertRuleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ProjectAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *projectAlertRuleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ProjectAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewProjectAlertRuleLifecycleAdapter(name string, clusterScoped bool, client ProjectAlertRuleInterface, l ProjectAlertRuleLifecycle) ProjectAlertRuleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ProjectAlertRuleGroupVersionResource)
	}
	adapter := &projectAlertRuleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ProjectAlertRule) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
