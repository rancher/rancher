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

type RKE1NodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
}

func (r *RKE1NodeDriverProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE1NodeDriverProvisioningTestSuite) SetupSuite() {
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

func (r *RKE1NodeDriverProvisioningTestSuite) TestProvisioningRKE1Cluster() {
	nodeRolesAll := []provisioninginput.NodePools{provisioninginput.AllRolesNodePool}
	nodeRolesShared := []provisioninginput.NodePools{provisioninginput.EtcdControlPlaneNodePool, provisioninginput.WorkerNodePool}
	nodeRolesDedicated := []provisioninginput.NodePools{provisioninginput.EtcdNodePool, provisioninginput.ControlPlaneNodePool, provisioninginput.WorkerNodePool}
	tests := []struct {
		name      string
		nodePools []provisioninginput.NodePools
		client    *rancher.Client
		runFlag   bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRolesAll, r.client, r.client.Flags.GetValue(environmentflag.Long)},
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), nodeRolesAll, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Short) || r.client.Flags.GetValue(environmentflag.Long)},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.AdminClientName.String(), nodeRolesShared, r.client, r.client.Flags.GetValue(environmentflag.Long)},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.StandardClientName.String(), nodeRolesShared, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Short) || r.client.Flags.GetValue(environmentflag.Long)},
		{"3 nodes - 1 role per node " + provisioninginput.AdminClientName.String(), nodeRolesDedicated, r.client, r.client.Flags.GetValue(environmentflag.Long)},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), nodeRolesDedicated, r.standardUserClient, r.client.Flags.GetValue(environmentflag.Long)},
	}
	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *r.provisioningConfig
		provisioningConfig.NodePools = tt.nodePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
	}
}

func (r *RKE1NodeDriverProvisioningTestSuite) TestProvisioningRKE1ClusterDynamicInput() {

	if len(r.provisioningConfig.NodePools) == 0 {
		r.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{provisioninginput.AdminClientName.String(), r.client},
		{provisioninginput.StandardClientName.String(), r.standardUserClient},
	}
	for _, tt := range tests {
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, r.provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1NodeDriverProvisioningTestSuite))
}
