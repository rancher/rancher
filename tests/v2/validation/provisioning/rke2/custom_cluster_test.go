package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/permutations"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CustomClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	clustersConfig     *provisioninginput.Config
	isWindows          bool
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	c.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, c.clustersConfig)

	c.isWindows = false
	for _, pool := range c.clustersConfig.NodesAndRoles {
		if pool.Windows {
			c.isWindows = true
			break
		}
	}

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	c.clustersConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(c.client, clusters.RKE2ClusterType.String(), c.clustersConfig.RKE2KubernetesVersions)
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
	nodeRolesAll := []machinepools.NodeRoles{provisioninginput.AllRolesPool}
	nodeRolesShared := []machinepools.NodeRoles{provisioninginput.EtcdControlPlanePool, provisioninginput.WorkerPool}
	nodeRolesDedicated := []machinepools.NodeRoles{provisioninginput.EtcdPool, provisioninginput.ControlPlanePool, provisioninginput.WorkerPool}
	nodeRolesDedicatedWindows := []machinepools.NodeRoles{provisioninginput.EtcdPool, provisioninginput.ControlPlanePool, provisioninginput.WorkerPool, provisioninginput.WindowsPool}

	tests := []struct {
		name      string
		client    *rancher.Client
		nodeRoles []machinepools.NodeRoles
		isWindows bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), c.client, nodeRolesAll, false},
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesAll, false},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.AdminClientName.String(), c.client, nodeRolesShared, false},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesShared, false},
		{"3 nodes - 1 role per node " + provisioninginput.AdminClientName.String(), c.client, nodeRolesDedicated, false},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicated, false},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioninginput.AdminClientName.String(), c.client, nodeRolesDedicatedWindows, true},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicatedWindows, true},
	}
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()
		if (c.isWindows == tt.isWindows) || (c.isWindows && !tt.isWindows) {
			permutations.RunTestPermutations(&c.Suite, tt.name, tt.client, c.clustersConfig, permutations.RKE2CustomCluster, nil, nil)
		} else {
			c.T().Skip("Skipping Windows tests")
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE2CustomClusterDynamicInput() {
	if len(c.clustersConfig.NodesAndRoles) == 0 {
		c.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{provisioninginput.AdminClientName.String(), c.client},
		{provisioninginput.StandardClientName.String(), c.standardUserClient},
	}
	for _, tt := range tests {
		testSession := session.NewSession()
		defer testSession.Cleanup()
		_, err := tt.client.WithSession(testSession)
		require.NoError(c.T(), err)

		permutations.RunTestPermutations(&c.Suite, tt.name, tt.client, c.clustersConfig, permutations.RKE2CustomCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
