//go:build validation

package provisioning

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters/aks"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HostedAKSClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	cluster            *management.Cluster
}

func (h *HostedAKSClusterProvisioningTestSuite) TearDownSuite() {
	h.session.Cleanup()
}

func (h *HostedAKSClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	h.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(h.T(), err)

	h.client = client

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
	require.NoError(h.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(h.T(), err)

	h.standardUserClient = standardUserClient
}

func (h *HostedAKSClusterProvisioningTestSuite) TestProvisioningHostedAKS() {
	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{provisioninginput.AdminClientName.String(), h.client},
		{provisioninginput.StandardClientName.String(), h.standardUserClient},
	}

	for _, tt := range tests {
		var aksClusterConfig aks.ClusterConfig
		config.LoadConfig(aks.AKSClusterConfigConfigurationFileKey, &aksClusterConfig)

		clusterObject, err := provisioning.CreateProvisioningAKSHostedCluster(tt.client, aksClusterConfig)
		reports.TimeoutRKEReport(clusterObject, err)
		require.NoError(h.T(), err)

		provisioning.VerifyHostedCluster(h.T(), tt.client, clusterObject)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHostedAKSClusterProvisioningTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/hosted-providers-e2e for updated tests")
	suite.Run(t, new(HostedAKSClusterProvisioningTestSuite))
}
