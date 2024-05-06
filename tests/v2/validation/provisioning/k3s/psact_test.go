//go:build validation

package k3s

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
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type K3SPSACTTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (k *K3SPSACTTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *K3SPSACTTestSuite) SetupSuite() {
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

func (k *K3SPSACTTestSuite) TestK3SPSACTNodeDriverCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMachinePool,
		provisioninginput.ControlPlaneMachinePool,
		provisioninginput.WorkerMachinePool,
	}

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		psact        provisioninginput.PSACT
		client       *rancher.Client
	}{
		{
			name:         "Rancher Privileged " + provisioninginput.StandardClientName.String(),
			machinePools: nodeRolesDedicated,
			psact:        "rancher-privileged",
			client:       k.standardUserClient,
		},
		{
			name:         "Rancher Restricted " + provisioninginput.StandardClientName.String(),
			machinePools: nodeRolesDedicated,
			psact:        "rancher-restricted",
			client:       k.standardUserClient,
		},
		{
			name:         "Rancher Baseline " + provisioninginput.AdminClientName.String(),
			machinePools: nodeRolesDedicated,
			psact:        "rancher-baseline",
			client:       k.client,
		},
	}

	for _, tt := range tests {
		provisioningConfig := *k.provisioningConfig
		provisioningConfig.MachinePools = tt.machinePools
		provisioningConfig.PSACT = string(tt.psact)
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, &provisioningConfig,
			permutations.K3SProvisionCluster, nil, nil)
	}
}

func (k *K3SPSACTTestSuite) TestK3SPSACTCustomCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMachinePool,
		provisioninginput.ControlPlaneMachinePool,
		provisioninginput.WorkerMachinePool,
	}

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		psact        provisioninginput.PSACT
		client       *rancher.Client
	}{
		{
			"Rancher Privileged " + provisioninginput.StandardClientName.String(),
			nodeRolesDedicated,
			"rancher-privileged",
			k.standardUserClient,
		},
		{
			"Rancher Restricted " + provisioninginput.StandardClientName.String(),
			nodeRolesDedicated,
			"rancher-restricted",
			k.standardUserClient,
		},
		{
			"Rancher Baseline " + provisioninginput.AdminClientName.String(),
			nodeRolesDedicated,
			"rancher-baseline",
			k.client,
		},
	}

	for _, tt := range tests {
		provisioningConfig := *k.provisioningConfig
		provisioningConfig.MachinePools = tt.machinePools
		provisioningConfig.PSACT = string(tt.psact)
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, &provisioningConfig,
			permutations.K3SCustomCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SPSACTTestSuite(t *testing.T) {
	suite.Run(t, new(K3SPSACTTestSuite))
}
