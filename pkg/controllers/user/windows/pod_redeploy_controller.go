package windows

import (
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	apicorev1 "github.com/rancher/types/apis/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// podRedeployController is to delete the pod doesn't have the tolerate configuration but its owner has it.
// We will only force delete those pod without tolerate configuration.
type podRedeployController struct {
	workloadHandler util.CommonController
	podClient       apicorev1.PodInterface
}

func (c *podRedeployController) sync(key string, obj *v1.Pod) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	if !canDeployedIntoLinuxNode(obj.Spec) {
		return obj, nil
	}

	owner, err := c.workloadHandler.GetWorkloadByOwnerReferences(obj.Namespace, obj.OwnerReferences)
	if err != nil {
		return obj, err
	}
	if owner == nil {
		return obj, nil
	}

	// don't do anything if the owner workload doesn't have the toleration
	if !tolerationExists(owner.TemplateSpec.Spec.Tolerations) {
		return obj, nil
	}

	// if the pod doesn't have the tolerations, we need to delete it and let the owner recreate it.
	if !tolerationExists(obj.Spec.Tolerations) {
		return nil, c.podClient.DeleteNamespaced(obj.Namespace, obj.Name, &metav1.DeleteOptions{})
	}

	return obj, nil
}
