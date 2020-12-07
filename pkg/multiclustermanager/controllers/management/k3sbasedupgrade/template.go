package k3sbasedupgrade

import (
	"strings"

	"github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/describe"
)

const k3sMasterPlanName = "k3s-master-plan"
const k3sWorkerPlanName = "k3s-worker-plan"
const systemUpgradeServiceAccount = "system-upgrade"
const k3supgradeImage = "rancher/k3s-upgrade"
const rke2upgradeImage = "rancher/rke2-upgrade"
const rke2MasterPlanName = "rke2-master-plan"
const rke2WorkerPlanName = "rke2-worker-plan"

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

func generateMasterPlan(version string, concurrency int, drain bool, upgradeImage, masterPlanName string) planv1.Plan {
	masterPlan := genericPlan
	masterPlan.Spec.Upgrade.Image = upgradeImage
	masterPlan.Name = masterPlanName
	masterPlan.Spec.Version = version
	masterPlan.Spec.Concurrency = int64(concurrency)

	if drain {
		masterPlan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	// only select master nodes
	masterPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{

			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		}},
	}

	return masterPlan
}

func generateWorkerPlan(version string, concurrency int, drain bool, upgradeImage, workerPlanName, masterPlanName string) planv1.Plan {
	workerPlan := genericPlan
	workerPlan.Spec.Upgrade.Image = upgradeImage
	workerPlan.Name = workerPlanName
	workerPlan.Spec.Version = version
	workerPlan.Spec.Concurrency = int64(concurrency)

	if drain {
		workerPlan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	// worker plans wait for master plans to complete
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		Image: upgradeImage + ":" + parseVersion(version),
		Args:  []string{"prepare", masterPlanName},
	}
	// select all nodes that are not master
	workerPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpDoesNotExist,
		}},
	}

	return workerPlan
}

func configureMasterPlan(masterPlan planv1.Plan, version string, concurrency int, drain bool, masterPlanName string) planv1.Plan {
	masterPlan.Name = masterPlanName
	masterPlan.Spec.Version = version
	masterPlan.Spec.Concurrency = int64(concurrency)

	if drain {
		masterPlan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	// only select master nodes
	masterPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{

			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		}},
	}
	return masterPlan
}

func configureWorkerPlan(workerPlan planv1.Plan, version string, concurrency int, drain bool, upgradeImage, workerPlanName, masterPlanName string) planv1.Plan {
	workerPlan.Name = workerPlanName
	workerPlan.Spec.Version = version
	workerPlan.Spec.Concurrency = int64(concurrency)

	if drain {
		workerPlan.Spec.Drain = &planv1.DrainSpec{
			Force: true,
		}
	}

	// worker plans wait for master plans to complete
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		Image: upgradeImage + ":" + parseVersion(version),
		Args:  []string{"prepare", masterPlanName},
	}

	workerPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpDoesNotExist,
		}},
	}

	return workerPlan
}

// a valid k3s version needs to be converted to valid docker tag (v1.17.3+k3s1 => v1.17.3-k3s1)
func parseVersion(v string) string {
	return strings.Replace(v, "+", "-", -1)
}
