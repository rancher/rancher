package provisioning

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ClusterID string

func (c *CustomClusterProvisioningTestSuite) setupHybrid() {
	c.SetupSuiteHybrid()
}

func (c *CustomClusterProvisioningTestSuite) ProvisioningRKE2CustomClusterHybrid(externalNodeProvider ExternalNodeProvider) {
	nodeRoles0 := []string{
		"--etcd --controlplane --worker",
	}

	nodeRoles1 := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}

	tests := []struct {
		name       string
		nodeRoles  []string
		hasWindows bool
		client     *rancher.Client
	}{
		{"1 Node all roles Admin User + 1 Windows Worker - Hybrid", nodeRoles0, true, c.client},
		{"1 Node all roles Standard User + 1 Windows Worker - Hybrid", nodeRoles0, true, c.standardUserClient},
		{"3 unique role nodes as Admin User + 1 Windows Worker - Hybrid", nodeRoles1, true, c.client},
		{"3 unique role nodes as Standard User + 1 Windows Worker - Hybrid", nodeRoles1, true, c.standardUserClient},
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

					ClusterID = customCluster.Status.ClusterName

					token, err := tokenregistration.GetRegistrationToken(client, ClusterID)
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

func (c *CustomClusterProvisioningTestSuite) ProvisioningRKE2CustomClusterWithDynamicInputHybrid(externalNodeProvider ExternalNodeProvider, nodesAndRoles []map[string]bool) {
	var rolesPerNode []string

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

					ClusterID = customCluster.Status.ClusterName
					token, err := tokenregistration.GetRegistrationToken(client, ClusterID)
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

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomClusterHybrid() {
	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := ExternalNodeProviderSetup(nodeProviderName)
		c.ProvisioningRKE2CustomClusterHybrid(externalNodeProvider)
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningCustomClusterDynamicInputHybrid() {
	nodesAndRoles := NodesAndRolesInput()
	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

	for _, nodeProviderName := range c.nodeProviders {
		externalNodeProvider := ExternalNodeProviderSetup(nodeProviderName)
		c.ProvisioningRKE2CustomClusterWithDynamicInputHybrid(externalNodeProvider, nodesAndRoles)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterProvisioningTestSuiteHybrid(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
