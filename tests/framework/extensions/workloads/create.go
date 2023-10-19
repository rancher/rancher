package workloads

import (
	v1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultNamespace = "default"
	port             = "port"
	ServiceType      = "service"
)

// CreateDeploymentWithService is a helper function to create a deployment and service in the downstream cluster.
func CreateDeploymentWithService(steveclient *steveV1.Client, wlName string, deployment *v1.Deployment, service corev1.Service) (*steveV1.SteveAPIObject, *steveV1.SteveAPIObject, error) {
	logrus.Infof("Creating deployment: %s", wlName)
	deploymentResp, err := steveclient.SteveType(DeploymentSteveType).Create(deployment)
	if err != nil {
		logrus.Errorf("Failed to create deployment: %s", wlName)
		return nil, nil, err
	}

	logrus.Infof("Successfully created deployment: %s", wlName)

	logrus.Infof("Creating service: %s", service.Name)
	serviceResp, err := steveclient.SteveType(ServiceType).Create(service)
	if err != nil {
		logrus.Errorf("Failed to create service: %s", service.Name)
		return nil, nil, err
	}

	logrus.Infof("Successfully created service: %s", service.Name)

	return deploymentResp, serviceResp, err
}
