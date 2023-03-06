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
	hardening "github.com/rancher/rancher/tests/framework/extensions/hardening/k3s"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
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
	provisioning       *provisioning.Config
	kubernetesVersions []string
	nodeProviders      []string
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.K3SKubernetesVersions
	c.nodeProviders = clustersConfig.NodeProviders
	c.provisioning = clustersConfig

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
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

func (c *CustomClusterProvisioningTestSuite) TestProvisioningK3SCustomCluster() {
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
		hardening *provisioning.Config
		client    *rancher.Client
	}{
		{"1 Node all roles Admin User", nodeRoles0, c.provisioning, c.client},
		{"1 Node all roles Standard User", nodeRoles0, c.provisioning, c.standardUserClient},
		{"3 nodes - 1 role per node Admin User", nodeRoles1, c.provisioning, c.client},
		{"3 nodes - 1 role per node Standard User", nodeRoles1, c.provisioning, c.standardUserClient},
	}
	var name string
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		for _, nodeProviderName := range c.nodeProviders {
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
			for _, kubeVersion := range c.kubernetesVersions {
				name = tt.name + " Kubernetes version: " + kubeVersion
				c.Run(name, func() {
					c.testProvisioningK3SCustomCluster(client, externalNodeProvider, tt.nodeRoles, kubeVersion, tt.hardening)
				})
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningK3SCustomClusterDynamicInput() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

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

	tests := []struct {
		name      string
		client    *rancher.Client
		hardening *provisioning.Config
	}{
		{"Admin User", c.client, c.provisioning},
		{"Standard User", c.standardUserClient, c.provisioning},
	}
	var name string
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		for _, nodeProviderName := range c.nodeProviders {
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
			for _, kubeVersion := range c.kubernetesVersions {
				name = tt.name + " Kubernetes version: " + kubeVersion
				c.Run(name, func() {
					c.testProvisioningK3SCustomCluster(client, externalNodeProvider, rolesPerNode, kubeVersion, tt.hardening)
				})
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) testProvisioningK3SCustomCluster(client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []string, kubeVersion string, harden *provisioning.Config) {
	namespace := "fleet-default"

	numNodes := len(nodesAndRoles)
	nodes, _, err := externalNodeProvider.NodeCreationFunc(client, numNodes, 0, false)
	require.NoError(c.T(), err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

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
		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, nodesAndRoles[key])

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

	if harden.Hardened && kubeVersion < "v1.25.0" {
		err = hardening.HardeningNodes(client, harden.Hardened, nodes, nodesAndRoles)
		require.NoError(c.T(), err)

		hardenCluster := clusters.HardenK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, nil)

		hardenClusterResp, err := clusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
		require.NoError(c.T(), err)
		assert.Equal(c.T(), clusterName, hardenClusterResp.ObjectMeta.Name)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
