//go:build (validation || sanity) && !infra.any && !infra.aks && !infra.eks && !infra.rke2k3s && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !extended && !stress

package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/validation/provisioning/permutations"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
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
		runFlag      bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), c.client, nodeRolesAll, false, c.client.Flags.GetValue(environmentflag.Long)},
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesAll, false, c.client.Flags.GetValue(environmentflag.Short) || c.client.Flags.GetValue(environmentflag.Long)},
		{"2 nodes - etcd|cp roles per 1 node " + provisioninginput.AdminClientName.String(), c.client, nodeRolesShared, false, c.client.Flags.GetValue(environmentflag.Long)},
		{"2 nodes - etcd|cp roles per 1 node " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesShared, false, c.client.Flags.GetValue(environmentflag.Short) || c.client.Flags.GetValue(environmentflag.Long)},
		{"3 nodes - 1 role per node " + provisioninginput.AdminClientName.String(), c.client, nodeRolesDedicated, false, c.client.Flags.GetValue(environmentflag.Long)},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicated, false, c.client.Flags.GetValue(environmentflag.Long)},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioninginput.AdminClientName.String(), c.client, nodeRolesDedicatedWindows, true, c.client.Flags.GetValue(environmentflag.Long)},
		{"4 nodes - 1 role per node + 1 windows worker" + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicatedWindows, true, c.client.Flags.GetValue(environmentflag.Long)},
	}
	for _, tt := range tests {
		if !tt.runFlag {
			c.T().Logf("SKIPPED")
			continue
		}

		testSession := session.NewSession()
		defer testSession.Cleanup()
		if (c.isWindows == tt.isWindows) || (c.isWindows && !tt.isWindows) {
			provisioningConfig := *c.provisioningConfig
			provisioningConfig.MachinePools = tt.machinePools
			permutations.RunTestPermutations(&c.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE2CustomCluster, nil, nil)
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
