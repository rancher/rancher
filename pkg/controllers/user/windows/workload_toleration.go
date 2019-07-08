package windows

import (
	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

var (
	HostOSLabels = []labels.Set{
		labels.Set(map[string]string{
			"beta.kubernetes.io/os": "linux",
		}),
		labels.Set(map[string]string{
			"kubernetes.io/os": "linux",
		}),
	}
)

// WorkloadTolerationHandler add toleration to the workload with the node selector indicates that
// this workload should be only deployed linux node.
type WorkloadTolerationHandler struct {
	workloadController util.CommonController
}

func (w *WorkloadTolerationHandler) sync(key string, obj *util.Workload) error {
	if obj == nil {
		return nil
	}
	if !canDeployedIntoLinuxNode(obj) {
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
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.Deployments.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case rc != nil:
		if tolerationExists(rc.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := rc.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.ReplicationControllers.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case rs != nil:
		if tolerationExists(rs.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := rs.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.ReplicaSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case ds != nil:
		if tolerationExists(ds.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := ds.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.DaemonSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case ss != nil:
		if tolerationExists(ss.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := ss.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.StatefulSets.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case job != nil:
		if tolerationExists(job.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := job.DeepCopy()
		newObj.Spec.Template.Spec.Tolerations = append(newObj.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.Jobs.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	case cj != nil:
		if tolerationExists(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) {
			return nil
		}
		newObj := cj.DeepCopy()
		newObj.Spec.JobTemplate.Spec.Template.Spec.Tolerations = append(newObj.Spec.JobTemplate.Spec.Template.Spec.Tolerations, newOSTolerations())
		if _, err := w.workloadController.CronJobs.Update(newObj); err != nil {
			return errors.Wrapf(err, "failed to update workload %s with os toleration", obj.Name)
		}
	}
	return nil
}

func canDeployedIntoLinuxNode(obj *util.Workload) bool {
	var targetSelectors []labels.Selector
	if obj.TemplateSpec.Spec.NodeSelector != nil {
		targetSelectors = append(targetSelectors, labels.Set(obj.TemplateSpec.Spec.NodeSelector).AsSelector())
	}
	if obj.TemplateSpec.Spec.Affinity != nil &&
		obj.TemplateSpec.Spec.Affinity.NodeAffinity != nil &&
		obj.TemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, terms := range obj.TemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, req := range terms.MatchExpressions {
				selector, err := helper.NodeSelectorRequirementsAsSelector([]v1.NodeSelectorRequirement{req})
				if err != nil {
					logrus.Warnf("failed to create selector from workload node affinity, error: %s", err.Error())
					continue
				}
				targetSelectors = append(targetSelectors, selector)
			}
		}
	}

	for _, selector := range targetSelectors {
		for _, hostOSLabel := range HostOSLabels {
			if selector.Matches(hostOSLabel) {
				return true
			}
		}
	}

	return false
}

func tolerationExists(tolerations []v1.Toleration) bool {
	return helper.TolerationsTolerateTaint(tolerations, &nodeTaint)
}

func newOSTolerations() v1.Toleration {
	return v1.Toleration{
		Key:      nodeTaint.Key,
		Value:    nodeTaint.Value,
		Operator: v1.TolerationOpEqual,
		Effect:   nodeTaint.Effect,
	}
}
