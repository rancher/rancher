package k3s

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type K3SNodeDriverDeleteTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	providers          []string
	psact              string
	advancedOptions    provisioning.AdvancedOptions
}

func (k *K3SNodeDriverDeleteTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *K3SNodeDriverDeleteTestSuite) SetupSuite() {
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

func (k *K3SNodeDriverDeleteTestSuite) TestDeletingK3SCluster() {
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
					cluster, _, _, err := TestProvisioningK3SCluster(k.T(), client, provider, nodesAndRoles, kubeVersion, tt.psact, k.advancedOptions)
					require.NoError(k.T(), err)

					TestDeletingK3SCluster(k.T(), client, cluster)
				})
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestK3SNodeDriverDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(K3SNodeDriverDeleteTestSuite))
}
