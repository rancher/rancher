package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CustomClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	hardened           bool
	kubernetesVersions []string
	cnis               []string
	nodeProviders      []string
	psact              string
	isWindows          bool
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

	c.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders
	c.hardened = clustersConfig.Hardened
	c.psact = clustersConfig.PSACT
	c.advancedOptions = clustersConfig.AdvancedOptions
	c.isWindows = false
	for _, pool := range clustersConfig.NodesAndRoles {
		if pool.Windows {
			c.isWindows = true
			break
		}
	}

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	c.kubernetesVersions, err = kubernetesversions.Default(c.client, clusters.RKE2ClusterType.String(), c.kubernetesVersions)
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

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE2CustomCluster() {
	nodeRolesAll := []machinepools.NodeRoles{provisioning.AllRolesPool}
	nodeRolesShared := []machinepools.NodeRoles{provisioning.EtcdControlPlanePool, provisioning.WorkerPool}
	nodeRolesDedicated := []machinepools.NodeRoles{provisioning.EtcdPool, provisioning.ControlPlanePool, provisioning.WorkerPool}
	nodeRolesDedicatedWindows := []machinepools.NodeRoles{provisioning.EtcdPool, provisioning.ControlPlanePool, provisioning.WorkerPool, provisioning.WindowsPool}

	tests := []struct {
		name      string
		client    *rancher.Client
		nodeRoles []machinepools.NodeRoles
		psact     string
		isWindows bool
	}{
		{"1 Node all roles " + provisioning.AdminClientName.String(), c.client, nodeRolesAll, c.psact, false},
		{"1 Node all roles " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRolesAll, c.psact, false},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.AdminClientName.String(), c.client, nodeRolesShared, c.psact, false},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRolesShared, c.psact, false},
		{"3 nodes - 1 role per node " + provisioning.AdminClientName.String(), c.client, nodeRolesDedicated, c.psact, false},
		{"3 nodes - 1 role per node " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRolesDedicated, c.psact, false},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioning.AdminClientName.String(), c.client, nodeRolesDedicatedWindows, c.psact, true},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioning.StandardClientName.String(), c.standardUserClient, nodeRolesDedicatedWindows, c.psact, true},
	}
	var name string
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()

		client, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)
		if (c.isWindows == tt.isWindows) || (c.isWindows && !tt.isWindows) {
			for _, nodeProviderName := range c.nodeProviders {
				externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)
				providerName := " Node Provider: " + nodeProviderName
				for _, kubeVersion := range c.kubernetesVersions {
					name = tt.name + providerName + " Kubernetes version: " + kubeVersion
					for _, cni := range c.cnis {
						name += " cni: " + cni
						c.Run(name, func() {
							TestProvisioningRKE2CustomCluster(c.T(), client, externalNodeProvider, tt.nodeRoles, tt.psact, kubeVersion, cni, c.hardened, c.advancedOptions)
						})
					}
				}
			}
		} else {
			c.T().Skip("Skipping Windows tests")
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
			providerName := " Node Provider: " + nodeProviderName
			for _, kubeVersion := range c.kubernetesVersions {
				name = tt.name + providerName + " Kubernetes version: " + kubeVersion
				for _, cni := range c.cnis {
					name += " cni: " + cni
					c.Run(name, func() {
						TestProvisioningRKE2CustomCluster(c.T(), client, externalNodeProvider, nodesAndRoles, tt.psact, kubeVersion, cni, c.hardened, c.advancedOptions)
					})
				}
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
