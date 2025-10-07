package deployment

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	appsv1 "k8s.io/api/apps/v1"
)

func fetchCurrentDeploymentHash(deployment *appsv1.Deployment) string {
	if deployment == nil {
		return ""
	}

	currentLabels := deployment.GetLabels()
	if currentHash, ok := currentLabels[consts.LabelSccOperatorHash]; ok {
		return currentHash
	}

	return ""
}
