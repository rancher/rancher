package k3sbasedupgrade

import (
	"strings"

	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	"github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/describe"
)

const (
	k3sMasterPlanName           = "k3s-master-plan"
	k3sWorkerPlanName           = "k3s-worker-plan"
	systemUpgradeServiceAccount = "system-upgrade-controller"
	k3supgradeImage             = "rancher/k3s-upgrade"
	rke2upgradeImage            = "rancher/rke2-upgrade"
	rke2MasterPlanName          = "rke2-master-plan"
	rke2WorkerPlanName          = "rke2-worker-plan"
)

var genericPlan = planv1.Plan{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Plan",
		APIVersion: upgrade.GroupName + `/v1`,
	},
	ObjectMeta: metav1.ObjectMeta{
		Namespace: systemUpgradeNS,
		Labels:    map[string]string{rancherManagedPlan: "true"},
	},
	Spec: planv1.PlanSpec{
		Concurrency:        0,
		ServiceAccountName: systemUpgradeServiceAccount,
		Cordon:             true,
		Upgrade:            &planv1.ContainerSpec{},
	},
	Status: planv1.PlanStatus{},
}

func generateMasterPlan(version string, concurrency int, drain bool, image, masterPlanName string) planv1.Plan {
	masterPlan := genericPlan
	return generatePlan(masterPlan, masterPlanName, version, image, concurrency, drain, &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		},
			{
				Key:      nodesyncer.UpgradeEnabledLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
		},
	})
}

func generateWorkerPlan(version string, concurrency int, drain bool, image, workerPlanName, masterPlanName string) planv1.Plan {
	workerPlan := genericPlan
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		Image: image + ":" + parseVersion(version),
		Args:  []string{"prepare", masterPlanName},
	}
	return generatePlan(workerPlan, workerPlanName, version, image, concurrency, drain, &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      describe.LabelNodeRolePrefix + "master",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			},
			{
				Key:      nodesyncer.UpgradeEnabledLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
		},
	})
}

func configureMasterPlan(masterPlan planv1.Plan, version string, concurrency int, drain bool, masterPlanName string) planv1.Plan {
	return configurePlan(masterPlan, version, concurrency, drain, masterPlanName, &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      describe.LabelNodeRolePrefix + "master",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
			{
				Key:      nodesyncer.UpgradeEnabledLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
		},
	})
}

func configureWorkerPlan(workerPlan planv1.Plan, version string, concurrency int, drain bool, image, workerPlanName, masterPlanName string) planv1.Plan {
	// worker plans wait for master plans to complete
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		Image: image + ":" + parseVersion(version),
		Args:  []string{"prepare", masterPlanName},
	}
	return configurePlan(workerPlan, version, concurrency, drain, workerPlanName, &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      describe.LabelNodeRolePrefix + "master",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			},
			{
				Key:      nodesyncer.UpgradeEnabledLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"true"},
			},
		},
	})
}

func generatePlan(plan planv1.Plan, name, version, image string, concurrency int, drain bool, selector *metav1.LabelSelector) planv1.Plan {
	plan.Spec.Upgrade.Image = image
	plan.Name = name
	plan.Spec.Version = version
	plan.Spec.Concurrency = int64(concurrency)

	if drain {
		plan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	plan.Spec.NodeSelector = selector

	plan.Spec.Tolerations = []corev1.Toleration{{
		Operator: corev1.TolerationOpExists,
	}}

	return plan
}

func configurePlan(plan planv1.Plan, version string, concurrency int, drain bool, name string, selector *metav1.LabelSelector) planv1.Plan {
	plan.Name = name
	plan.Spec.Version = version
	plan.Spec.Concurrency = int64(concurrency)

	if drain {
		plan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	plan.Spec.NodeSelector = selector

	plan.Spec.Tolerations = []corev1.Toleration{{
		Operator: corev1.TolerationOpExists,
	}}

	plan.Spec.ServiceAccountName = genericPlan.Spec.ServiceAccountName

	return plan
}

// a valid k3s version needs to be converted to valid docker tag (v1.17.3+k3s1 => v1.17.3-k3s1)
func parseVersion(v string) string {
	return strings.Replace(v, "+", "-", -1)
}
