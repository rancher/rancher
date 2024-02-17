//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.rke2k3s && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package rke1

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

type RKE1CloudProviderTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
}

func (r *RKE1CloudProviderTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE1CloudProviderTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.provisioningConfig.RKE1KubernetesVersions, err = kubernetesversions.Default(
		r.client, clusters.RKE1ClusterType.String(), r.provisioningConfig.RKE1KubernetesVersions)
	require.NoError(r.T(), err)

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

func (r *RKE1CloudProviderTestSuite) TestAWSCloudProviderRKE1Cluster() {
	nodeRolesDedicated := []provisioninginput.NodePools{provisioninginput.EtcdNodePool, provisioninginput.ControlPlaneNodePool, provisioninginput.WorkerNodePool}
	nodeRolesDedicated[0].NodeRoles.Quantity = 3
	nodeRolesDedicated[1].NodeRoles.Quantity = 2
	nodeRolesDedicated[2].NodeRoles.Quantity = 2

	tests := []struct {
		name      string
		nodePools []provisioninginput.NodePools
		client    *rancher.Client
		runFlag   bool
	}{
		{"OutOfTree" + provisioninginput.StandardClientName.String(), nodeRolesDedicated, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Long)},
	}
	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *r.provisioningConfig
		provisioningConfig.CloudProvider = "external-aws"
		provisioningConfig.NodePools = tt.nodePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1CloudProviderTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1CloudProviderTestSuite))
}
