package Context

import (
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	provisionClient "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
)

type ManagmentContxt struct {
	Client *managementClient.Client
	User   *managementClient.User
}

type ClusterContext struct {
	ClusterOperations provisionClient.ClusterInterface
	Cluster           *apisV1.Cluster
}

type ProjectContext struct {
	ProjetOperations managementClient.ProjectOperations
	Project          *managementClient.Project
}
