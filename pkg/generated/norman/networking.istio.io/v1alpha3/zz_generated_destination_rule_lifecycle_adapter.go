package v1alpha3

import (
	"github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type DestinationRuleLifecycle interface {
	Create(obj *v1alpha3.DestinationRule) (runtime.Object, error)
	Remove(obj *v1alpha3.DestinationRule) (runtime.Object, error)
	Updated(obj *v1alpha3.DestinationRule) (runtime.Object, error)
}

type destinationRuleLifecycleAdapter struct {
	lifecycle DestinationRuleLifecycle
}

func (w *destinationRuleLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *destinationRuleLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *destinationRuleLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v1alpha3.DestinationRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *destinationRuleLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v1alpha3.DestinationRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *destinationRuleLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v1alpha3.DestinationRule))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewDestinationRuleLifecycleAdapter(name string, clusterScoped bool, client DestinationRuleInterface, l DestinationRuleLifecycle) DestinationRuleHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(DestinationRuleGroupVersionResource)
	}
	adapter := &destinationRuleLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v1alpha3.DestinationRule) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
