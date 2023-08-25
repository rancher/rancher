package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
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

type RKE2ACETestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	clustersConfig     *provisioninginput.Config
}

func (r *RKE2ACETestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2ACETestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

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
	nodeRoles0 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     3,
		},
		{
			ControlPlane: false,
			Etcd:         true,
			Worker:       false,
			Quantity:     1,
		},
		{
			ControlPlane: false,
			Etcd:         false,
			Worker:       true,
			Quantity:     1,
		},
	}

	tests := []struct {
		name      string
		nodeRoles []machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"Multiple Control Planes - Admin", nodeRoles0, r.client},
		{"Multiple Control Planes - Standard", nodeRoles0, r.standardUserClient},
	}
	require.NotNil(r.T(), r.clustersConfig.Networking.LocalClusterAuthEndpoint)
	// Test is obsolete when ACE is not set.
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)
		r.clustersConfig.NodesAndRoles = tt.nodeRoles
		permutations.RunTestPermutations(&r.Suite, tt.name, client, r.clustersConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func TestRKE2ACETestSuite(t *testing.T) {
	suite.Run(t, new(RKE2ACETestSuite))
}
