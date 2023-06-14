package rke2

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

type RKE2NodeDriverDeleteProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	providers          []string
	psact              string
	advancedOptions    provisioning.AdvancedOptions
}

func (r *RKE2NodeDriverDeleteProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2NodeDriverDeleteProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.psact = clustersConfig.PSACT
	r.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.kubernetesVersions, err = kubernetesversions.Default(r.client, clusters.RKE2ClusterType.String(), r.kubernetesVersions)
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

func (r *RKE2NodeDriverDeleteProvisioningTestSuite) TestDeletingRKE2Cluster() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		r.T().Skip()
	}

	tests := []struct {
		name   string
		client *rancher.Client
		psact  string
	}{
		{provisioning.AdminClientName.String(), r.client, r.psact},
		{provisioning.StandardClientName.String(), r.standardUserClient, r.psact},
	}

	var name string
	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()
		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)

		for _, providerName := range r.providers {
			provider := CreateProvider(providerName)
			providerName := " Node Provider: " + provider.Name.String()
			for _, kubeVersion := range r.kubernetesVersions {
				name = tt.name + providerName + " Kubernetes version: " + kubeVersion
				for _, cni := range r.cnis {
					name += " cni: " + cni
					r.Run(name, func() {
						cluster, _, _, err := TestProvisioningRKE2Cluster(r.T(), client, provider, nodesAndRoles, kubeVersion, cni, tt.psact, r.advancedOptions)
						require.NoError(r.T(), err)

						TestDeletingRKE2Cluster(r.T(), client, cluster)
					})
				}
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2NodeDriverDeleteTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverDeleteProvisioningTestSuite))
}
