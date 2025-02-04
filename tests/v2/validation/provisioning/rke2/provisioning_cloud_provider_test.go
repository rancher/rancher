//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package rke2

import (
	"slices"
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

type RKE2CloudProviderTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (r *RKE2CloudProviderTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2CloudProviderTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)
	r.client = client

	if r.provisioningConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.Default(r.client, clusters.RKE2ClusterType.String(), nil)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	} else if r.provisioningConfig.RKE2KubernetesVersions[0] == "all" {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(r.client)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE2KubernetesVersions = rke2Versions
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
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *RKE2CloudProviderTestSuite) TestAWSCloudProviderCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated[0].MachinePoolConfig.Quantity = 3
	nodeRolesDedicated[1].MachinePoolConfig.Quantity = 2
	nodeRolesDedicated[2].MachinePoolConfig.Quantity = 2

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		runFlag      bool
	}{
		{"OutOfTree" + provisioninginput.StandardClientName.String(), nodeRolesDedicated, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Long)},
	}

	if !slices.Contains(r.provisioningConfig.Providers, "aws") {
		r.T().Skip("AWS Cloud Provider test requires access to aws.")
	}

	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}

		provisioningConfig := *r.provisioningConfig
		provisioningConfig.CloudProvider = "aws"
		provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func (r *RKE2CloudProviderTestSuite) TestVsphereCloudProviderCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated[0].MachinePoolConfig.Quantity = 3
	nodeRolesDedicated[1].MachinePoolConfig.Quantity = 2
	nodeRolesDedicated[2].MachinePoolConfig.Quantity = 2

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		runFlag      bool
	}{
		{"OutOfTree" + provisioninginput.StandardClientName.String(), nodeRolesDedicated, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Long)},
	}

	if !slices.Contains(r.provisioningConfig.Providers, "vsphere") {
		r.T().Skip("Vsphere Cloud Provider test requires access to vsphere.")
	}

	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}

		provisioningConfig := *r.provisioningConfig
		provisioningConfig.CloudProvider = "rancher-vsphere"
		provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2CloudProviderTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2CloudProviderTestSuite))
}
