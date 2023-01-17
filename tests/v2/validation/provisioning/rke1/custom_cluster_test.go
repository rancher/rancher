package rke1

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
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
	kubernetesVersions []string
	cnis               []string
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

	c.kubernetesVersions = clustersConfig.RKE1KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders

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

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE1CustomCluster() {
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
		testSession := session.NewSession(c.T())
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		for _, nodeProviderName := range c.nodeProviders {
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
			for _, kubeVersion := range c.kubernetesVersions {
				name = tt.name + " Kubernetes version: " + kubeVersion
				for _, cni := range c.cnis {
					name += " cni: " + cni
					c.Run(name, func() {
						c.testProvisioningRKE1CustomCluster(client, externalNodeProvider, tt.nodeRoles, kubeVersion, cni)
					})
				}
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE1CustomClusterDynamicInput() {
	rolesPerNode := []string{}

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1

	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

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
		name   string
		client *rancher.Client
	}{
		{"Admin User", c.client},
		{"Standard User", c.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		testSession := session.NewSession(c.T())
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		for _, nodeProviderName := range c.nodeProviders {
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
			for _, kubeVersion := range c.kubernetesVersions {
				name = tt.name + " Kubernetes version: " + kubeVersion
				for _, cni := range c.cnis {
					name += " cni: " + cni
					c.Run(name, func() {
						c.testProvisioningRKE1CustomCluster(client, externalNodeProvider, rolesPerNode, kubeVersion, cni)
					})
				}
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) testProvisioningRKE1CustomCluster(client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []string, kubeVersion string, cni string) {
	numNodes := len(nodesAndRoles)
	nodes, _, err := externalNodeProvider.NodeCreationFunc(client, numNodes, 0, false)
	require.NoError(c.T(), err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, client)

	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(c.T(), err)

	client, err = client.ReLogin()
	require.NoError(c.T(), err)

	customCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	require.NoError(c.T(), err)

	token, err := tokenregistration.GetRegistrationToken(client, customCluster.ID)
	require.NoError(c.T(), err)

	for key, node := range nodes {
		c.T().Logf("Execute Registration Command for node %s", node.NodeID)
		command := fmt.Sprintf("%s %s", token.NodeCommand, nodesAndRoles[key])

		output, err := node.ExecuteCommand(command)
		require.NoError(c.T(), err)
		c.T().Logf(output)
	}

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}
	watchInterface, err := c.client.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(c.T(), err)

	checkFunc := clusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(c.T(), err)
	assert.Equal(c.T(), clusterName, clusterResp.Name)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(c.T(), err)
	assert.NotEmpty(c.T(), clusterToken)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
