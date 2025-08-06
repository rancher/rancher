package deployment

import (
	"github.com/rancher/rancher/pkg/scc/consts"
	appsControllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fetchCurrentDeployment(deployments appsControllers.DeploymentController) (*appsv1.Deployment, error) {
	currentDeployment, err := deployments.Get(consts.DefaultSCCNamespace, consts.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return currentDeployment, nil
}

func fetchCurrentDeploymentHash(deployments appsControllers.DeploymentController) string {
	deployment, err := fetchCurrentDeployment(deployments)
	if err != nil {
		return ""
	}

	currentLabels := deployment.GetLabels()
	if currentHash, ok := currentLabels[consts.LabelSccOperatorHash]; ok {
		return currentHash
	}

	return ""
}
