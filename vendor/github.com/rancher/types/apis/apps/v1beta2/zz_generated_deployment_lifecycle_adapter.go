package v1beta2

import (
	"github.com/rancher/norman/lifecycle"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
)

type DeploymentLifecycle interface {
	Create(obj *v1beta2.Deployment) error
	Remove(obj *v1beta2.Deployment) error
	Updated(obj *v1beta2.Deployment) error
}

type deploymentLifecycleAdapter struct {
	lifecycle DeploymentLifecycle
}

func (w *deploymentLifecycleAdapter) Create(obj runtime.Object) error {
	return w.lifecycle.Create(obj.(*v1beta2.Deployment))
}

func (w *deploymentLifecycleAdapter) Finalize(obj runtime.Object) error {
	return w.lifecycle.Remove(obj.(*v1beta2.Deployment))
}

func (w *deploymentLifecycleAdapter) Updated(obj runtime.Object) error {
	return w.lifecycle.Updated(obj.(*v1beta2.Deployment))
}

func NewDeploymentLifecycleAdapter(name string, client DeploymentInterface, l DeploymentLifecycle) DeploymentHandlerFunc {
	adapter := &deploymentLifecycleAdapter{lifecycle: l}
	syncFn := lifecycle.NewObjectLifecycleAdapter(name, adapter, client.ObjectClient())
	return func(key string, obj *v1beta2.Deployment) error {
		if obj == nil {
			return syncFn(key, nil)
		}
		return syncFn(key, obj)
	}
}
