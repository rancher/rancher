package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAlertRuleLifecycle interface {
	Create(obj *ClusterAlertRule) (runtime.Object, error)
	Remove(obj *ClusterAlertRule) (runtime.Object, error)
	Updated(obj *ClusterAlertRule) (runtime.Object, error)
}

type clusterAlertRuleLifecycleAdapter struct {
	lifecycle ClusterAlertRuleLifecycle
}

func (w *clusterAlertRuleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *clusterAlertRuleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *clusterAlertRuleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*ClusterAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertRuleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*ClusterAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *clusterAlertRuleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*ClusterAlertRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewClusterAlertRuleLifecycleAdapter(name string, clusterScoped bool, client ClusterAlertRuleInterface, l ClusterAlertRuleLifecycle) ClusterAlertRuleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(ClusterAlertRuleGroupVersionResource)
	}
	adapter := &clusterAlertRuleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *ClusterAlertRule) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
