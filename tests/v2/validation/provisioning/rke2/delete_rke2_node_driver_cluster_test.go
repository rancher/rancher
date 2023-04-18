package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE2NodeDriverDeletingTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	providers          []string
	nodeProviders      []string
}

func (r *RKE2NodeDriverDeletingTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2NodeDriverDeletingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.nodeProviders = clustersConfig.NodeProviders
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

func (r *RKE2NodeDriverDeletingTestSuite) TestCreateAndDeleteRKE2Cluster() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		r.T().Skip()
	}

	var name string
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	for index, providerName := range r.providers {
		provider := CreateProvider(providerName)
		externaNodeProvider := provisioning.ExternalNodeProviderSetup(r.nodeProviders[index])
		for _, kubeVersion := range r.kubernetesVersions {
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					CreateAndDeleteRKE2Cluster(r.T(), client, provider, nodesAndRoles, kubeVersion, cni, &externaNodeProvider)
				})
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2DeletingTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverDeletingTestSuite))
}
