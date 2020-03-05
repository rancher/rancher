package k3supgrade

import (
	"github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/describe"
)

const k3sMasterPlanName = "k3s-master-plan"
const k3sWorkerPlanName = "k3s-worker-plan"
const systemUpgradeServiceAccount = "system-upgrade"
const upgradeImage = "rancher/k3s-upgrade"

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
		Channel:            "",
		Version:            "",
		Secrets:            nil,
		Prepare:            nil,
		Cordon:             false,
		Drain: &planv1.DrainSpec{
			Force: true,
		},
		Upgrade: &planv1.ContainerSpec{
			Image: upgradeImage,
		},
	},
	Status: planv1.PlanStatus{},
}

func generateMasterPlan(version string, concurrency int) planv1.Plan {
	masterPlan := genericPlan
	masterPlan.Name = k3sMasterPlanName
	masterPlan.Spec.Version = version
	masterPlan.Spec.Concurrency = int64(concurrency)

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

func generateWorkerPlan(version string, concurrency int) planv1.Plan {
	workerPlan := genericPlan
	workerPlan.Name = k3sWorkerPlanName
	workerPlan.Spec.Version = version
	workerPlan.Spec.Concurrency = int64(concurrency)

	// worker plans wait for master plans to complete
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		// TODO: the image version doesn't matter here, needs something recent
		Image: upgradeImage + ":" + "v1.17.3-k3s1",
		Args:  []string{"prepare", k3sMasterPlanName},
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

func configureMasterPlan(masterPlan planv1.Plan, version string, concurrency int) planv1.Plan {
	masterPlan.Name = k3sMasterPlanName
	masterPlan.Spec.Version = version
	masterPlan.Spec.Concurrency = int64(concurrency)

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

func configureWorkerPlan(workerPlan planv1.Plan, version string, concurrency int) planv1.Plan {
	workerPlan.Name = k3sWorkerPlanName
	workerPlan.Spec.Version = version
	workerPlan.Spec.Concurrency = int64(concurrency)

	// worker plans wait for master plans to complete
	workerPlan.Spec.Prepare = &planv1.ContainerSpec{
		// TODO need a recent valid image tag here...
		Image: upgradeImage + ":" + "v1.17.3-k3s1",
		Args:  []string{"prepare", k3sMasterPlanName},
	}

	workerPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      describe.LabelNodeRolePrefix + "master",
			Operator: metav1.LabelSelectorOpDoesNotExist,
		}},
	}

	return workerPlan
}
