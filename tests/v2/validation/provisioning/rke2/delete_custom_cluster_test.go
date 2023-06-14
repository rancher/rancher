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

type CustomClusterDeleteTestSuite struct {
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

func (c *CustomClusterDeleteTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterDeleteTestSuite) SetupSuite() {
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

func (c *CustomClusterDeleteTestSuite) TestDeletingRKE2CustomCluster() {
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
						cluster, err := TestProvisioningRKE2CustomCluster(c.T(), client, externalNodeProvider, nodesAndRoles, tt.psact, kubeVersion, cni, c.hardened, c.advancedOptions)
						require.NoError(c.T(), err)

						TestDeletingRKE2Cluster(c.T(), client, cluster)
					})
				}
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterDeleteTestSuite))
}
