package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
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
	provisioningConfig *provisioninginput.Config
	isWindows          bool
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	c.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, c.provisioningConfig)

	c.isWindows = false
	for _, pool := range c.provisioningConfig.MachinePools {
		if pool.NodeRoles.Windows {
			c.isWindows = true
			break
		}
	}

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	c.provisioningConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(c.client, clusters.RKE2ClusterType.String(), c.provisioningConfig.RKE2KubernetesVersions)
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
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	nodeRolesShared := []provisioninginput.MachinePools{provisioninginput.EtcdControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicatedWindows := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool, provisioninginput.WindowsMachinePool}

	tests := []struct {
		name         string
		client       *rancher.Client
		machinePools []provisioninginput.MachinePools
		isWindows    bool
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
			c.provisioningConfig.MachinePools = tt.machinePools
			permutations.RunTestPermutations(&c.Suite, tt.name, tt.client, c.provisioningConfig, permutations.RKE2CustomCluster, nil, nil)
		} else {
			c.T().Skip("Skipping Windows tests")
		}
	}
}

func (c *CustomClusterProvisioningTestSuite) TestProvisioningRKE2CustomClusterDynamicInput() {
	if len(c.provisioningConfig.MachinePools) == 0 {
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

		permutations.RunTestPermutations(&c.Suite, tt.name, tt.client, c.provisioningConfig, permutations.RKE2CustomCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCustomClusterRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClusterProvisioningTestSuite))
}
