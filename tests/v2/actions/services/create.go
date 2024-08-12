package services

import (
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// CreateService is a helper function to create a service in the downstream cluster
func CreateService(steveclient *steveV1.Client, service corev1.Service) (*steveV1.SteveAPIObject, error) {
	logrus.Infof("Creating service: %s", service.Name)
	serviceResp, err := steveclient.SteveType(ServiceSteveType).Create(service)
	if err != nil {
		return nil, err
	}

	return serviceResp, err
}
