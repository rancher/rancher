package rke2

import (
	"context"
	"fmt"
	"testing"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	hardening "github.com/rancher/rancher/tests/framework/extensions/hardening/rke2"
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
	cnis               []string
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

	c.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	c.cnis = clustersConfig.CNIs
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

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE2CustomCluster() {
	nodeRoles0 := []string{
		"--etcd --controlplane --worker",
	}

	nodeRoles1 := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}

	tests := []struct {
		name         string
		client       *rancher.Client
		nodeRoles    []string
		hardening    *provisioning.Config
		nodeCountWin int
		hasWindows   bool
	}{
		{"1 Node all roles Admin User", c.client, nodeRoles0, c.provisioning, 0, false},
		{"1 Node all roles Standard User", c.standardUserClient, nodeRoles0, c.provisioning, 0, false},
		{"3 nodes - 1 role per node Admin User", c.client, nodeRoles1, c.provisioning, 0, false},
		{"3 nodes - 1 role per node Standard User", c.standardUserClient, nodeRoles1, c.provisioning, 0, false},
		{"1 Node all roles Admin User + 1 Windows Worker", c.client, nodeRoles0, c.provisioning, 1, true},
		{"1 Node all roles Standard User + 1 Windows Worker", c.standardUserClient, nodeRoles0, c.provisioning, 1, true},
		{"3 nodes - 1 role per node Admin User + 2 Windows Workers", c.client, nodeRoles1, c.provisioning, 2, true},
		{"3 nodes - 1 role per node Standard User + 2 Windows Workers", c.standardUserClient, nodeRoles1, c.provisioning, 2, true},
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
				for _, cni := range c.cnis {
					name += " cni: " + cni
					c.Run(name, func() {
						c.testProvisioningRKE2CustomCluster(client, externalNodeProvider, tt.nodeRoles, kubeVersion, cni, tt.hardening, tt.nodeCountWin, tt.hasWindows)
					})
				}
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE2CustomClusterDynamicInput() {
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
		name         string
		client       *rancher.Client
		hardening    *provisioning.Config
		nodeCountWin int
		hasWindows   bool
	}{
		{"Admin User", c.client, c.provisioning, 0, false},
		{"Standard User", c.standardUserClient, c.provisioning, 0, false},
		{"Admin User + 1 Windows Worker", c.client, c.provisioning, 1, false},
		{"Standard User + 1 Windows Worker", c.standardUserClient, c.provisioning, 1, false},
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
				for _, cni := range c.cnis {
					name += " cni: " + cni
					c.Run(name, func() {
						c.testProvisioningRKE2CustomCluster(client, externalNodeProvider, rolesPerNode, kubeVersion, cni, tt.hardening, tt.nodeCountWin, tt.hasWindows)
					})
				}
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) testProvisioningRKE2CustomCluster(client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []string, kubeVersion, cni string, harden *provisioning.Config, nodeCountWin int, hasWindows bool) {
	numNodesLin := len(nodesAndRoles)

	linuxNodes, winNodes, err := externalNodeProvider.NodeCreationFunc(client, numNodesLin, nodeCountWin, hasWindows)
	require.NoError(c.T(), err)

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, nil)

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

	for key, linuxNode := range linuxNodes {
		c.T().Logf("Execute Registration Command for node %s", linuxNode.NodeID)
		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, nodesAndRoles[key])

		output, err := linuxNode.ExecuteCommand(command)
		require.NoError(c.T(), err)

		c.T().Logf(output)
	}

	kubeProvisioningClient, err := c.client.GetKubeAPIProvisioningClient()
	require.NoError(c.T(), err)
	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + clusterName,

		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(c.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady
	err = wait.WatchWait(result, checkFunc)
	assert.NoError(c.T(), err)
	assert.Equal(c.T(), clusterName, clusterResp.ObjectMeta.Name)

	if hasWindows {
		for _, winNode := range winNodes {
			c.T().Logf("Execute Registration Command for node %s", winNode.NodeID)
			winCommand := fmt.Sprintf("%s", token.InsecureWindowsNodeCommand)

			output, err := winNode.ExecuteCommand("powershell.exe" + winCommand)
			require.NoError(c.T(), err)

			c.T().Logf(string(output[:]))
		}

		kubeWinProvisioningClient, err := c.client.GetKubeAPIProvisioningClient()
		require.NoError(c.T(), err)

		result, err := kubeWinProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector: "metadata.name=" + clusterName,

			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		require.NoError(c.T(), err)

		checkFunc := clusters.IsProvisioningClusterReady
		err = wait.WatchWait(result, checkFunc)
		assert.NoError(c.T(), err)
		assert.Equal(c.T(), clusterName, clusterResp.ObjectMeta.Name)
	}

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(c.T(), err)
	assert.NotEmpty(c.T(), clusterToken)

	if harden.Hardened && kubeVersion < "v1.25.0" {
		err = hardening.HardeningNodes(client, harden.Hardened, linuxNodes, nodesAndRoles)
		require.NoError(c.T(), err)

		hardenCluster := clusters.HardenK3SRKE2ClusterConfig(clusterName, namespace, "", "", kubeVersion, nil)

		hardenClusterResp, err := clusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
		require.NoError(c.T(), err)
		assert.Equal(c.T(), clusterName, hardenClusterResp.ObjectMeta.Name)

		err = hardening.PostHardeningConfig(client, harden.Hardened, linuxNodes, nodesAndRoles)
		require.NoError(c.T(), err)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
