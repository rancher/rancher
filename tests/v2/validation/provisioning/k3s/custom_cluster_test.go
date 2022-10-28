package k3s

import (
	"context"
	"fmt"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	nodeProviders      []string
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession(c.T())
	c.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.KubernetesVersions
	c.nodeProviders = clustersConfig.NodeProviders

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	enabled := true
	var testuser = provisioning.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(c.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(c.T(), err)

	c.standardUserClient = standardUserClient
}

func (c *CustomClusterProvisioningTestSuite) ProvisioningK3SCustomCluster(externalNodeProvider provisioning.ExternalNodeProvider) {
	namespace := "fleet-default"

	nodeRoles0 := []string{
		"--etcd --controlplane --worker",
	}

	nodeRoles1 := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}

	tests := []struct {
		name      string
		nodeRoles []string
		client    *rancher.Client
	}{
		{"1 Node all roles Admin User", nodeRoles0, c.client},
		{"1 Node all roles Standard User", nodeRoles0, c.standardUserClient},
		{"3 nodes - 1 role per node Admin User", nodeRoles1, c.client},
		{"3 nodes - 1 role per node Standard User", nodeRoles1, c.standardUserClient},
	}
	var name string
	for _, tt := range tests {
		for _, kubeVersion := range c.kubernetesVersions {
			name = tt.name + " Kubernetes version: " + kubeVersion
			c.Run(name, func() {
				testSession := session.NewSession(c.T())
				defer testSession.Cleanup()

				client, err := tt.client.WithSession(testSession)
				require.NoError(c.T(), err)

				numNodes := len(tt.nodeRoles)
				nodes, err := externalNodeProvider.NodeCreationFunc(client, numNodes)
				require.NoError(c.T(), err)

				clusterName := provisioning.AppendRandomString(externalNodeProvider.Name)

				cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, nil)

				clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
				require.NoError(c.T(), err)

				client, err = client.ReLogin()
				require.NoError(c.T(), err)

				customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResouceType).ByID(clusterResp.ID)
				require.NoError(c.T(), err)

				clusterStatus := &apiv1.ClusterStatus{}
				err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
				require.NoError(c.T(), err)

				token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
				require.NoError(c.T(), err)

				for key, node := range nodes {
					c.T().Logf("Execute Registration Command for node %s", node.NodeID)
					command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, tt.nodeRoles[key])

					output, err := node.ExecuteCommand(command)
					require.NoError(c.T(), err)
					c.T().Logf(output)
				}

				kubeProvisioningClient, err := c.client.GetKubeAPIProvisioningClient()
				require.NoError(c.T(), err)

				result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector:  "metadata.name=" + clusterName,
					TimeoutSeconds: &defaults.WatchTimeoutSeconds,
				})
				require.NoError(c.T(), err)

				checkFunc := clusters.IsProvisioningClusterReady

				err = wait.WatchWait(result, checkFunc)
				assert.NoError(c.T(), err)
				assert.Equal(c.T(), clusterName, clusterResp.ObjectMeta.Name)

				clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
				require.NoError(c.T(), err)
				assert.NotEmpty(c.T(), clusterToken)
			})
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) ProvisioningK3SCustomClusterDynamicInput(externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []machinepools.NodeRoles) {
	namespace := "fleet-default"

	rolesPerNode := []string{}

	for _, nodes := range nodesAndRoles {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}
		rolesPerNode = append(rolesPerNode, finalRoleCommand)
	}

	numOfNodes := len(rolesPerNode)

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Admin User", c.client},
		{"Standard User", c.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		for _, kubeVersion := range c.kubernetesVersions {
			name = tt.name + " Kubernetes version: " + kubeVersion
			c.Run(name, func() {
				testSession := session.NewSession(c.T())
				defer testSession.Cleanup()

				client, err := tt.client.WithSession(testSession)
				require.NoError(c.T(), err)

				nodes, err := externalNodeProvider.NodeCreationFunc(client, numOfNodes)
				require.NoError(c.T(), err)

				clusterName := provisioning.AppendRandomString(externalNodeProvider.Name)

				cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, nil)

				clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
				require.NoError(c.T(), err)

				client, err = client.ReLogin()
				require.NoError(c.T(), err)

				customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResouceType).ByID(clusterResp.ID)
				require.NoError(c.T(), err)

				clusterStatus := &apiv1.ClusterStatus{}
				err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
				require.NoError(c.T(), err)

				token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
				require.NoError(c.T(), err)

				for key, node := range nodes {
					c.T().Logf("Execute Registration Command for node %s", node.NodeID)
					command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, rolesPerNode[key])

					output, err := node.ExecuteCommand(command)
					require.NoError(c.T(), err)
					c.T().Logf(output)
				}

				kubeProvisioningClient, err := c.client.GetKubeAPIProvisioningClient()
				require.NoError(c.T(), err)

				result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector:  "metadata.name=" + clusterName,
					TimeoutSeconds: &defaults.WatchTimeoutSeconds,
				})
				require.NoError(c.T(), err)

				checkFunc := clusters.IsProvisioningClusterReady

				err = wait.WatchWait(result, checkFunc)
				assert.NoError(c.T(), err)
				assert.Equal(c.T(), clusterName, clusterResp.ObjectMeta.Name)

				clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
				require.NoError(c.T(), err)
				assert.NotEmpty(c.T(), clusterToken)
			})
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomCluster() {
	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
		c.ProvisioningK3SCustomCluster(externalNodeProvider)
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomClusterDynamicInput() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
		c.ProvisioningK3SCustomClusterDynamicInput(externalNodeProvider, nodesAndRoles)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
