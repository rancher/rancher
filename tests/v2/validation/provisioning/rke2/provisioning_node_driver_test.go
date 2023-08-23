package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
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

type RKE2NodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	clustersConfig     *provisioninginput.Config
}

func (r *RKE2NodeDriverProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2NodeDriverProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)
	r.client = client

	r.clustersConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(
		r.client, clusters.RKE2ClusterType.String(), r.clustersConfig.RKE2KubernetesVersions)
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

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningRKE2Cluster() {
	nodeRoles0 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         true,
			Worker:       true,
			Quantity:     1,
		},
	}

	nodeRoles1 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
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

	nodeRoles2 := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     1,
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
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRoles0, r.client},
		{"1 Node all roles " + provisioninginput.StandardClientName.String(), nodeRoles0, r.standardUserClient},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.AdminClientName.String(), nodeRoles1, r.client},
		{"2 nodes - etcd/cp roles per 1 node " + provisioninginput.StandardClientName.String(), nodeRoles1, r.standardUserClient},
		{"3 nodes - 1 role per node " + provisioninginput.AdminClientName.String(), nodeRoles2, r.client},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), nodeRoles2, r.standardUserClient},
	}

	for _, tt := range tests {
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, r.clustersConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningRKE2ClusterDynamicInput() {
	if len(r.clustersConfig.NodesAndRoles) == 0 {
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
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, r.clustersConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverProvisioningTestSuite))
}
