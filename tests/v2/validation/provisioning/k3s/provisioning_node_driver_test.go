//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.rke2k3s && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package k3s

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
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

	if k.provisioningConfig.K3SKubernetesVersions == nil {
		k3sVersions, err := kubernetesversions.Default(k.client, clusters.K3SClusterType.String(), nil)
		require.NoError(k.T(), err)

		k.provisioningConfig.K3SKubernetesVersions = k3sVersions
	} else if k.provisioningConfig.K3SKubernetesVersions[0] == "all" {
		k3sVersions, err := kubernetesversions.ListK3SAllVersions(k.client)
		require.NoError(k.T(), err)

		k.provisioningConfig.K3SKubernetesVersions = k3sVersions
	}

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
		runFlag      bool
	}{
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), nodeRolesAll, k.standardUserClient, k.client.Flags.GetValue(environmentflag.Short) || k.client.Flags.GetValue(environmentflag.Long)},
		{"2 nodes - etcd|cp roles per 1 node " + provisioninginput.StandardClientName.String(), nodeRolesShared, k.standardUserClient, k.client.Flags.GetValue(environmentflag.Short) || k.client.Flags.GetValue(environmentflag.Long)},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), nodeRolesDedicated, k.standardUserClient, k.client.Flags.GetValue(environmentflag.Long)},
	}

	for _, tt := range tests {
		if !tt.runFlag {
			k.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *k.provisioningConfig
		provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
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
