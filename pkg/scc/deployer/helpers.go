package deployer

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	deploymentControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fetchCurrentDeployment(deployments deploymentControllers.DeploymentController) (*appsv1.Deployment, error) {
	sccDeployment, err := deployments.Get(consts.DefaultSCCNamespace, consts.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return sccDeployment, nil
}

func fetchCurrentDeploymentHash(deployments deploymentControllers.DeploymentController) string {
	deployment, err := fetchCurrentDeployment(deployments)
	if err != nil {
		return ""
	}

	currentLabels := deployment.GetLabels()
	currentHash, ok := currentLabels[consts.LabelSccOperatorHash]
	if ok {
		return currentHash
	}

	return ""
}
