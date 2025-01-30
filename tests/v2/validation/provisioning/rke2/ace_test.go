//go:build validation

package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE2ACETestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (r *RKE2ACETestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2ACETestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	if r.provisioningConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.Default(r.client, clusters.RKE2ClusterType.String(), nil)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	} else if r.provisioningConfig.RKE2KubernetesVersions[0] == "all" {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(r.client)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	}

	r.client = client

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

func (r *RKE2ACETestSuite) TestProvisioningRKE2ClusterACE() {
	nodeRoles0 := []provisioninginput.MachinePools{
		{
			MachinePoolConfig: machinepools.MachinePoolConfig{
				NodeRoles: machinepools.NodeRoles{
					ControlPlane: true,
					Etcd:         false,
					Worker:       false,
					Quantity:     3,
				},
			},
		},
		{
			MachinePoolConfig: machinepools.MachinePoolConfig{
				NodeRoles: machinepools.NodeRoles{
					ControlPlane: false,
					Etcd:         true,
					Worker:       false,
					Quantity:     1,
				},
			},
		},
		{
			MachinePoolConfig: machinepools.MachinePoolConfig{
				NodeRoles: machinepools.NodeRoles{
					ControlPlane: false,
					Etcd:         false,
					Worker:       true,
					Quantity:     1,
				},
			},
		},
	}

	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
	}{
		{"Multiple Control Planes - Standard", nodeRoles0, r.standardUserClient},
	}
	require.NotNil(r.T(), r.provisioningConfig.Networking.LocalClusterAuthEndpoint)
	// Test is obsolete when ACE is not set.
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)
		r.provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&r.Suite, tt.name, client, r.provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func TestRKE2ACETestSuite(t *testing.T) {
	suite.Run(t, new(RKE2ACETestSuite))
}
