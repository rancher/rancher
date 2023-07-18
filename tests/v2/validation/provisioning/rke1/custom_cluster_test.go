package rke1

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CustomClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	nodeProviders      []string
	psact              string
	advancedOptions    provisioning.AdvancedOptions
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.RKE1KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders
	c.psact = clustersConfig.PSACT
	c.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	c.kubernetesVersions, err = kubernetesversions.Default(c.client, clusters.RKE1ClusterType.String(), c.kubernetesVersions)
	require.NoError(c.T(), err)

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
	nodeRolesAll := []nodepools.NodeRoles{provisioning.RKE1AllRolesPool}
	nodeRolesShared := []nodepools.NodeRoles{provisioning.RKE1EtcdControlPlanePool, provisioning.RKE1WorkerPool}
	nodeRolesDedicated := []nodepools.NodeRoles{provisioning.RKE1EtcdPool, provisioning.RKE1ControlPlanePool, provisioning.RKE1WorkerPool}

	require.GreaterOrEqual(c.T(), len(c.cnis), 1)

	tests := []struct {
		name      string
		nodeRoles []nodepools.NodeRoles
		client    *rancher.Client
		psact     string
	}{
		{"1 Node all roles " + provisioning.AdminClientName.String(), nodeRolesAll, c.client, c.psact},
		{"1 Node all roles " + provisioning.StandardClientName.String(), nodeRolesAll, c.standardUserClient, c.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.AdminClientName.String(), nodeRolesShared, c.client, c.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.StandardClientName.String(), nodeRolesShared, c.standardUserClient, c.psact},
		{"3 nodes - 1 role per node " + provisioning.AdminClientName.String(), nodeRolesDedicated, c.client, c.psact},
		{"3 nodes - 1 role per node " + provisioning.StandardClientName.String(), nodeRolesDedicated, c.standardUserClient, c.psact},
	}

	var name string
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		for _, nodeProviderName := range c.nodeProviders {
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
			providerName := " Node Provider: " + nodeProviderName
			for _, kubeVersion := range c.kubernetesVersions {
				for _, cni := range c.cnis {
					name = tt.name + providerName + " Kubernetes version: " + kubeVersion + " cni: " + cni
					c.Run(name, func() {
						TestProvisioningRKE1CustomCluster(c.T(), client, externalNodeProvider, tt.nodeRoles, tt.psact, kubeVersion, cni, c.advancedOptions)
					})
				}
			}
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE1CustomClusterDynamicInput() {
	require.GreaterOrEqual(c.T(), len(c.cnis), 1)

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1

	if len(nodesAndRoles) == 0 {
		c.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
		psact  string
	}{
		{provisioning.AdminClientName.String(), c.client, c.psact},
		{provisioning.StandardClientName.String(), c.standardUserClient, c.psact},
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
				for _, cni := range c.cnis {
					name = tt.name + " Kubernetes version: " + kubeVersion + " cni: " + cni
					c.Run(name, func() {
						TestProvisioningRKE1CustomCluster(c.T(), client, externalNodeProvider, nodesAndRoles, tt.psact, kubeVersion, cni, c.advancedOptions)
					})
				}
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
