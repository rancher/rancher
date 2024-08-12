package deployment

import (
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
)

const (
	active              = "active"
	defaultNamespace    = "default"
	port                = "port"
	DeploymentSteveType = "apps.deployment"
)

// CreateDeployment is a helper function to create a deployment in the downstream cluster
func CreateDeployment(steveclient *steveV1.Client, wlName string, deployment *v1.Deployment) (*steveV1.SteveAPIObject, error) {
	logrus.Infof("Creating deployment: %s", wlName)
	deploymentResp, err := steveclient.SteveType(DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}
