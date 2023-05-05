package k3s

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type K3SNodeDriverProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	providers          []string
	psact              string
	advancedOptions    provisioning.AdvancedOptions
}

func (k *K3SNodeDriverProvisioningTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *K3SNodeDriverProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	k.kubernetesVersions = clustersConfig.K3SKubernetesVersions
	k.providers = clustersConfig.Providers
	k.psact = clustersConfig.PSACT
	k.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client

	k.kubernetesVersions, err = kubernetesversions.Default(k.client, clusters.K3SClusterType.String(), k.kubernetesVersions)
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
		psact     string
	}{
		{"1 Node all roles " + provisioning.AdminClientName.String(), nodeRoles0, k.client, k.psact},
		{"1 Node all roles " + provisioning.StandardClientName.String(), nodeRoles0, k.standardUserClient, k.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.AdminClientName.String(), nodeRoles1, k.client, k.psact},
		{"2 nodes - etcd/cp roles per 1 node " + provisioning.StandardClientName.String(), nodeRoles1, k.standardUserClient, k.psact},
		{"3 nodes - 1 role per node " + provisioning.AdminClientName.String(), nodeRoles2, k.client, k.psact},
		{"3 nodes - 1 role per node " + provisioning.StandardClientName.String(), nodeRoles2, k.standardUserClient, k.psact},
	}

	var name string
	for _, tt := range tests {
		subSession := k.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(k.T(), err)

		for _, providerName := range k.providers {
			provider := CreateProvider(providerName)
			providerName := " Node Provider: " + provider.Name
			for _, kubeVersion := range k.kubernetesVersions {
				name = tt.name + providerName.String() + " Kubernetes version: " + kubeVersion
				k.Run(name, func() {
					TestProvisioningK3SCluster(k.T(), client, provider, tt.nodeRoles, kubeVersion, tt.psact, k.advancedOptions)
				})
			}
		}
	}
}

func (k *K3SNodeDriverProvisioningTestSuite) TestProvisioningK3SClusterDynamicInput() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		k.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
		psact  string
	}{
		{provisioning.AdminClientName.String(), k.client, k.psact},
		{provisioning.StandardClientName.String(), k.standardUserClient, k.psact},
	}

	var name string
	for _, tt := range tests {
		subSession := k.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(k.T(), err)

		for _, providerName := range k.providers {
			provider := CreateProvider(providerName)
			providerName := " Node Provider: " + provider.Name.String()
			for _, kubeVersion := range k.kubernetesVersions {
				name = tt.name + providerName + " Kubernetes version: " + kubeVersion
				k.Run(name, func() {
					TestProvisioningK3SCluster(k.T(), client, provider, nodesAndRoles, kubeVersion, tt.psact, k.advancedOptions)
				})
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(K3SNodeDriverProvisioningTestSuite))
}
