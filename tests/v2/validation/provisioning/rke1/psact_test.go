//go:build validation

package rke1

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
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE1PSACTTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (r *RKE1PSACTTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE1PSACTTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	if r.provisioningConfig.RKE1KubernetesVersions == nil {
		rke1Versions, err := kubernetesversions.Default(r.client, clusters.RKE1ClusterType.String(), nil)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE1KubernetesVersions = rke1Versions
	} else if r.provisioningConfig.RKE1KubernetesVersions[0] == "all" {
		rke1Versions, err := kubernetesversions.ListRKE1AllVersions(r.client)
		require.NoError(r.T(), err)

		r.provisioningConfig.RKE1KubernetesVersions = rke1Versions
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

func (r *RKE1PSACTTestSuite) TestRKE1PSACTNodeDriverCluster() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdNodePool,
		provisioninginput.ControlPlaneNodePool,
		provisioninginput.WorkerNodePool,
	}

	tests := []struct {
		name      string
		nodePools []provisioninginput.NodePools
		psact     provisioninginput.PSACT
		client    *rancher.Client
	}{
		{
			name:      "Rancher Privileged " + provisioninginput.StandardClientName.String(),
			nodePools: nodeRolesDedicated,
			psact:     "rancher-privileged",
			client:    r.standardUserClient,
		},
		{
			name:      "Rancher Restricted " + provisioninginput.StandardClientName.String(),
			nodePools: nodeRolesDedicated,
			psact:     "rancher-restricted",
			client:    r.standardUserClient,
		},
		{
			name:      "Rancher Baseline " + provisioninginput.AdminClientName.String(),
			nodePools: nodeRolesDedicated,
			psact:     "rancher-baseline",
			client:    r.client,
		},
	}

	for _, tt := range tests {
		provisioningConfig := *r.provisioningConfig
		provisioningConfig.NodePools = tt.nodePools
		provisioningConfig.PSACT = string(tt.psact)
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig,
			permutations.RKE1ProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1PSACTTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1PSACTTestSuite))
}
