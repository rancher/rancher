package provisioning

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
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

	clustersConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	enabled := true
	var testuser = AppendRandomString("testuser-")
	user := &management.User{
		Username: testuser,
		Password: "rancherrancher123!",
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(c.T(), err)

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(c.T(), err)

	c.standardUserClient = standardUserClient
}

func (c *CustomClusterProvisioningTestSuite) Provisioning_RKE2CustomCluster(externalNodeProvider ExternalNodeProvider) {
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
			for _, cni := range c.cnis {
				name += " cni: " + cni
				c.Run(name, func() {
					testSession := session.NewSession(c.T())
					defer testSession.Cleanup()

					client, err := c.client.WithSession(testSession)
					require.NoError(c.T(), err)

					numNodes := len(tt.nodeRoles)
					nodes, err := externalNodeProvider.NodeCreationFunc(client, numNodes)
					require.NoError(c.T(), err)

					clusterName := AppendRandomString(externalNodeProvider.Name)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, nil)

					clusterResp, err := clusters.CreateRKE2Cluster(client, cluster)
					require.NoError(c.T(), err)

					customCluster, err := client.Provisioning.Clusters(namespace).Get(context.TODO(), clusterResp.Name, metav1.GetOptions{})
					require.NoError(c.T(), err)

					token, err := tokenregistration.GetRegistrationToken(client, customCluster.Status.ClusterName)
					require.NoError(c.T(), err)

					for key, node := range nodes {
						c.T().Logf("Execute Registration Command for node %s", node.NodeID)
						command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, tt.nodeRoles[key])

						err = node.ExecuteCommand(command)
						require.NoError(c.T(), err)
					}

					result, err := client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(c.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(c.T(), err)
					assert.Equal(c.T(), clusterName, clusterResp.Name)
				})
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) Provisioning_RKE2CustomClusterDynamicInput(externalNodeProvider ExternalNodeProvider, nodesAndRoles []map[string]bool) {
	rolesPerNode := []string{}

	for _, nodes := range nodesAndRoles {
		var finalRoleCommand string
		for role := range nodes {
			finalRoleCommand += fmt.Sprintf(" --%s", role)
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
			for _, cni := range c.cnis {
				name += " cni: " + cni
				c.Run(name, func() {
					testSession := session.NewSession(c.T())
					defer testSession.Cleanup()

					client, err := c.client.WithSession(testSession)
					require.NoError(c.T(), err)

					nodes, err := externalNodeProvider.NodeCreationFunc(client, numOfNodes)
					require.NoError(c.T(), err)

					clusterName := AppendRandomString(externalNodeProvider.Name)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, "", kubeVersion, nil)

					clusterResp, err := clusters.CreateRKE2Cluster(client, cluster)
					require.NoError(c.T(), err)

					customCluster, err := client.Provisioning.Clusters(namespace).Get(context.TODO(), clusterResp.Name, metav1.GetOptions{})
					require.NoError(c.T(), err)

					token, err := tokenregistration.GetRegistrationToken(client, customCluster.Status.ClusterName)
					require.NoError(c.T(), err)

					for key, node := range nodes {
						c.T().Logf("Execute Registration Command for node %s", node.NodeID)
						command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, rolesPerNode[key])

						err = node.ExecuteCommand(command)
						require.NoError(c.T(), err)
					}

					result, err := client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(c.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(c.T(), err)
					assert.Equal(c.T(), clusterName, clusterResp.Name)
				})
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomCluster() {
	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := ExternalNodeProviderSetup(nodeProviderName)
		c.Provisioning_RKE2CustomCluster(externalNodeProvider)
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomClusterDynamicInput() {
	nodesAndRoles := NodesAndRolesInput()
	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := ExternalNodeProviderSetup(nodeProviderName)
		c.Provisioning_RKE2CustomClusterDynamicInput(externalNodeProvider, nodesAndRoles)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
