package eks

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/aws"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/eks"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HostedEKSClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
}

func (h *HostedEKSClusterProvisioningTestSuite) TearDownSuite() {
	h.session.Cleanup()
}

func (h *HostedEKSClusterProvisioningTestSuite) SetupSuite() {
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

func (h *HostedEKSClusterProvisioningTestSuite) TestProvisioningHostedEKS() {
	tests := []struct {
		name        string
		client      *rancher.Client
		clusterName string
	}{
		{provisioning.AdminClientName.String(), h.client, ""},
		{provisioning.StandardClientName.String(), h.standardUserClient, ""},
	}

	for _, tt := range tests {
		subSession := h.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(h.T(), err)

		h.Run(tt.name, func() {
			h.testProvisioningHostedEKSCluster(client, tt.clusterName)
		})
	}
}

func (h *HostedEKSClusterProvisioningTestSuite) testProvisioningHostedEKSCluster(rancherClient *rancher.Client, clusterName string) (*management.Cluster, error) {
	cloudCredential, err := aws.CreateAWSCloudCredentials(rancherClient)
	require.NoError(h.T(), err)

	clusterName = namegen.AppendRandomString("ekshostcluster")
	clusterResp, err := eks.CreateEKSHostedCluster(rancherClient, clusterName, cloudCredential.ID, false, false, false, false, map[string]string{})
	require.NoError(h.T(), err)

	if h.client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}
	watchInterface, err := h.client.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(h.T(), err)

	checkFunc := clusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(h.T(), err)
	assert.Equal(h.T(), clusterName, clusterResp.Name)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(rancherClient, clusterName)
	require.NoError(h.T(), err)
	assert.NotEmpty(h.T(), clusterToken)

	err = nodestat.IsNodeReady(rancherClient, clusterResp.ID)
	require.NoError(h.T(), err)

	podResults, podErrors := pods.StatusPods(rancherClient, clusterResp.ID)
	assert.NotEmpty(h.T(), podResults)
	assert.Empty(h.T(), podErrors)

	return clusterResp, nil
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHostedEKSClusterProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(HostedEKSClusterProvisioningTestSuite))
}
