package k3s

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

type K3SNodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (k *K3SNodeDriverProvisioningTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *K3SNodeDriverProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	k.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, k.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client

	k.provisioningConfig.K3SKubernetesVersions, err = kubernetesversions.Default(
		k.client, clusters.K3SClusterType.String(), k.provisioningConfig.K3SKubernetesVersions)
	require.NoError(k.T(), err)

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
	require.NoError(k.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(k.T(), err)

	k.standardUserClient = standardUserClient
}

func (k *K3SNodeDriverProvisioningTestSuite) TestProvisioningK3SCluster() {
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	nodeRolesShared := []provisioninginput.MachinePools{provisioninginput.EtcdControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRolesAll, k.client},
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), nodeRolesAll, k.standardUserClient},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.AdminClientName.String(), nodeRolesShared, k.client},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.StandardClientName.String(), nodeRolesShared, k.standardUserClient},
		{"3 nodes - 1 role per node " + provisioninginput.AdminClientName.String(), nodeRolesDedicated, k.client},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), nodeRolesDedicated, k.standardUserClient},
	}

	for _, tt := range tests {
		k.provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, k.provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
	}
}

func (k *K3SNodeDriverProvisioningTestSuite) TestProvisioningK3SClusterDynamicInput() {
	if len(k.provisioningConfig.MachinePools) == 0 {
		k.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{provisioninginput.AdminClientName.String(), k.client},
		{provisioninginput.StandardClientName.String(), k.standardUserClient},
	}

	for _, tt := range tests {
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, k.provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(K3SNodeDriverProvisioningTestSuite))
}
