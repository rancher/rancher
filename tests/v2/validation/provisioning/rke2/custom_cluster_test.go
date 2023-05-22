package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
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
	nodeRoles0 := []string{
		"--etcd --controlplane --worker",
	}

	nodeRoles1 := []string{
		"--etcd --controlplane",
		"--worker",
	}

	nodeRoles2 := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}

	tests := []struct {
		name         string
		client       *rancher.Client
		nodeRoles    []string
		nodeCountWin int
		hasWindows   bool
		psact        string
	}{
		{"1 Node all roles " + provisioning.AdminClientName.String(), c.client, nodeRoles0, 0, false, c.psact},
		{"1 Node all roles " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRoles0, 0, false, c.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.AdminClientName.String(), c.client, nodeRoles1, 0, false, c.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRoles1, 0, false, c.psact},
		{"3 nodes - 1 role per node " + provisioning.AdminClientName.String(), c.client, nodeRoles2, 0, false, c.psact},
		{"3 nodes - 1 role per node " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRoles2, 0, false, c.psact},
		{provisioning.AdminClientName.String() + " 1 Node all roles + 1 Windows Worker", c.client, nodeRoles0, 1, true, c.psact},
		{provisioning.StandardClientName.String() + " 1 Node all roles + 1 Windows Worker", c.standardUserClient, nodeRoles0, 1, true, c.psact},
		{provisioning.AdminClientName.String() + " 2 nodes - etcd/cp roles per 1 node + 1 Windows Worker", c.client, nodeRoles1, 1, true, c.psact},
		{provisioning.StandardClientName.String() + " 2 nodes - etcd/cp roles per 1 node + 1 Windows Worker", c.standardUserClient, nodeRoles1, 1, true, c.psact},
		{"3 nodes - 1 role per node " + provisioning.AdminClientName.String(), c.client, nodeRoles2, 2, true, c.psact},
		{"3 nodes - 1 role per node " + provisioning.StandardClientName.String(), c.standardUserClient, nodeRoles2, 2, true, c.psact},
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
						TestProvisioningRKE2CustomCluster(c.T(), client, externalNodeProvider, tt.nodeRoles, tt.psact, kubeVersion, cni, c.hardened, tt.nodeCountWin, tt.hasWindows, c.advancedOptions)
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
		nodeCountWin int
		hasWindows   bool
		psact        string
	}{
		{provisioning.AdminClientName.String(), c.client, 0, false, c.psact},
		{provisioning.StandardClientName.String(), c.standardUserClient, 0, false, c.psact},
		{"1 Windows Worker" + provisioning.AdminClientName.String(), c.client, 1, true, c.psact},
		{"1 Windows Worker" + provisioning.StandardClientName.String(), c.standardUserClient, 1, true, c.psact},
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
						TestProvisioningRKE2CustomCluster(c.T(), client, externalNodeProvider, rolesPerNode, tt.psact, kubeVersion, cni, c.hardened, tt.nodeCountWin, tt.hasWindows, c.advancedOptions)
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
