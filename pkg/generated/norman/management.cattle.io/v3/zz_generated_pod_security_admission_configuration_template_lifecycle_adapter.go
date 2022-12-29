package v3

import (
	"github.com/rancher/norman/lifecycle"
	"github.com/rancher/norman/resource"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodSecurityAdmissionConfigurationTemplateLifecycle interface {
	Create(obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error)
	Remove(obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error)
	Updated(obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error)
}

type podSecurityAdmissionConfigurationTemplateLifecycleAdapter struct {
	lifecycle PodSecurityAdmissionConfigurationTemplateLifecycle
}

func (w *podSecurityAdmissionConfigurationTemplateLifecycleAdapter) HasCreate() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasCreate()
}

func (w *podSecurityAdmissionConfigurationTemplateLifecycleAdapter) HasFinalize() bool {
	o, ok := w.lifecycle.(lifecycle.ObjectLifecycleCondition)
	return !ok || o.HasFinalize()
}

func (w *podSecurityAdmissionConfigurationTemplateLifecycleAdapter) Create(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Create(obj.(*v3.PodSecurityAdmissionConfigurationTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityAdmissionConfigurationTemplateLifecycleAdapter) Finalize(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Remove(obj.(*v3.PodSecurityAdmissionConfigurationTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func (w *podSecurityAdmissionConfigurationTemplateLifecycleAdapter) Updated(obj runtime.Object) (runtime.Object, error) {
	o, err := w.lifecycle.Updated(obj.(*v3.PodSecurityAdmissionConfigurationTemplate))
	if o == nil {
		return nil, err
	}
	return o, err
}

func NewPodSecurityAdmissionConfigurationTemplateLifecycleAdapter(name string, clusterScoped bool, client PodSecurityAdmissionConfigurationTemplateInterface, l PodSecurityAdmissionConfigurationTemplateLifecycle) PodSecurityAdmissionConfigurationTemplateHandlerFunc {
	if clusterScoped {
		resource.PutClusterScoped(PodSecurityAdmissionConfigurationTemplateGroupVersionResource)
	}
	adapter := &podSecurityAdmissionConfigurationTemplateLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, clusterScoped, adapter, client.ObjectClient())
	return func(key string, obj *v3.PodSecurityAdmissionConfigurationTemplate) (runtime.Object, error) {
		newObj, err := syncFn(key, obj)
		if o, ok := newObj.(runtime.Object); ok {
			return o, err
		}
		return nil, err
	}
}
