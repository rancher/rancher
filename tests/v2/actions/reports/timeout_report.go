package reports

import (
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
)

// TimeoutRKEReport is a function that print the State and Conditions of a RKE cluster if the error is a timeout error
func TimeoutRKEReport(cluster *management.Cluster, err error) {
	if err != nil && cluster != nil && err.Error() == wait.TimeoutError {
		logrus.Errorf("Timeout report State: %s, Conditions %v", cluster.State, cluster.Conditions)
	}
}

// TimeoutClusterReport is a function that print the State of a RKE2 or K3S cluster if the error is a timeout error
func TimeoutClusterReport(cluster *steveV1.SteveAPIObject, err error) {
	if err != nil && cluster != nil && err.Error() == wait.TimeoutError {
		logrus.Errorf("Timeout report State: %v", cluster.State)
	}
}
