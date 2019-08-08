package windows

import (
	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/taints"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

// WorkloadTolerationHandler add toleration to the workload with the node selector indicates that
// this workload should be only deployed linux node.
type WorkloadTolerationHandler struct {
	workloadController util.CommonController
}

func (w *WorkloadTolerationHandler) sync(key string, obj *util.Workload) error {
	if obj == nil || obj.TemplateSpec == nil {
		return nil
	}
	if !canDeployedIntoLinuxNode(obj.TemplateSpec.Spec) {
		return nil
	}
	// if the workload is controlled by other resources, we should not update the toleration.
	if len(obj.OwnerReferences) > 0 {
		return nil
	}
	dp, rc, rs, ds, ss, job, cj, err := w.workloadController.GetActualFromWorkload(obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get actual workload %s from lister", obj.Name)
	}
	switch {
	case dp != nil:
		if tolerationExists(dp.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := dp.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.Deployments.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case rc != nil:
		if tolerationExists(rc.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := rc.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.ReplicationControllers.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case rs != nil:
		if tolerationExists(rs.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := rs.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.ReplicaSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case ds != nil:
		if tolerationExists(ds.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := ds.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.DaemonSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case ss != nil:
		if tolerationExists(ss.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := ss.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.StatefulSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case job != nil:
		if tolerationExists(job.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := job.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.Jobs.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case cj != nil:
		if tolerationExists(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := cj.DeepCopy()
		newObj.Spec.JobTemplate.Spec.Template.Spec.Tolerations = append(newObj.Spec.JobTemplate.Spec.Template.Spec.Tolerations, taints.GetTolerationByTaint(taints.NodeTaint))
		if _, err := w.workloadController.CronJobs.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	}
	return nil
}

func canDeployedIntoLinuxNode(podSpec v1.PodSpec) bool {
	targetSelectors := map[string][]labels.Selector{}
	if podSpec.Affinity != nil &&
		podSpec.Affinity.NodeAffinity != nil &&
		podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		targetSelectors = taints.GetSelectorByNodeSelectorTerms(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)
	}

	for key, value := range podSpec.NodeSelector {
		targetSelectors[key] = append(targetSelectors[key], labels.Set(map[string]string{key: value}).AsSelector())
	}

	return taints.CanDeployToLinuxHost(targetSelectors)
}

func tolerationExists(tolerations []v1.Toleration) bool {
	return helper.TolerationsTolerateTaint(tolerations, &taints.NodeTaint)
}
