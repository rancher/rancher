package Context

import (
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	provisionClient "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/automation-framework/testclient"
)

type ManagmentContext struct {
	Client *testclient.Client
	User   *managementClient.User
}

type ClusterContext struct {
	Management        ManagmentContext
	ClusterOperations provisionClient.ClusterInterface
	Cluster           *apisV1.Cluster
}

type ProjectContext struct {
	Cluster          ClusterContext
	ProjetOperations managementClient.ProjectOperations
	Project          *managementClient.Project
}
