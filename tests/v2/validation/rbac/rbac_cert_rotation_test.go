package rbac

import (
	"context"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/certs/rotatecerts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CertRotationRbacTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUser       *management.User
	standardUserClient *rancher.Client
	session            *session.Session
	cluster            *management.Cluster
	adminProject       *management.Project
	namespace          string
	clusterName        string
}

func (rb *CertRotationRbacTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *CertRotationRbacTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rb.session = testSession

	rb.namespace = defaultNamespace

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client
	rb.clusterName = client.RancherConfig.ClusterName
	logrus.Infof("Cluster is %v", rb.clusterName)
	require.NotEmptyf(rb.T(), rb.clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, rb.clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)
}

func (rb *CertRotationRbacTestSuite) ValidateCertRotationCluster(role string) {

	log.Infof("Rotating Certificates of the cluster as %v", role)
	err := rotatecerts.RotateCertificates(rb.standardUserClient, rb.clusterName, rb.namespace)
	switch role {
	case roleOwner, restrictedAdmin:
		require.NoError(rb.T(), err)

	case roleMember, roleProjectOwner, roleProjectMember:
		require.Error(rb.T(), err)
		assert.Equal(rb.T(), "Resource type [provisioning.cattle.io.cluster] is not updatable", err.Error())
	}

	kubeProvisioningClient, err := rb.client.GetKubeAPIProvisioningClient()
	require.NoError(rb.T(), err)

	result, err := kubeProvisioningClient.Clusters(rb.namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + rb.clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(rb.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(result, checkFunc)
	assert.NoError(rb.T(), err)
}

func (rb *CertRotationRbacTestSuite) TestCertRotationRbac() {
	clusterID, err := clusters.GetClusterIDByName(rb.client, rb.clusterName)
	logrus.Infof("Cluster is %v", clusterID)
	require.NoError(rb.T(), err)
	if !(strings.Contains(clusterID, "c-m-")) {
		rb.T().Skip("Skipping tests since cluster is not of type - k3s or RKE2")
	}
	tests := []struct {
		name   string
		role   string
		member string
	}{
		{"Cluster Member", roleMember, standardUser},
		{"Project Owner", roleProjectOwner, standardUser},
		{"Project Member", roleProjectMember, standardUser},
		{"Restricted Admin", restrictedAdmin, restrictedAdmin},
		{"Cluster Owner", roleOwner, standardUser},
	}
	for _, tt := range tests {
		rb.Run("Set up User with Cluster Role "+tt.name, func() {
			newUser, err := createUser(rb.client, tt.member)
			require.NoError(rb.T(), err)
			rb.standardUser = newUser
			rb.T().Logf("Created user: %v", rb.standardUser.Username)
			rb.standardUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)

			subSession := rb.session.NewSession()
			defer subSession.Cleanup()

			createProjectAsAdmin, err := createProject(rb.client, rb.cluster.ID)
			rb.adminProject = createProjectAsAdmin
			require.NoError(rb.T(), err)
		})

		rb.Run("Adding user as "+tt.name+" to the downstream cluster.", func() {

			if tt.member == standardUser {
				if strings.Contains(tt.role, "project") {
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.standardUser, tt.role)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.role)
					require.NoError(rb.T(), err)
				}
			}

			relogin, err := rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)
			rb.standardUserClient = relogin
		})

		rb.T().Logf("Starting validations for %v", tt.role)

		rb.Run("Test case - RotateCertificates of a cluster as a "+tt.name, func() {
			rb.ValidateCertRotationCluster(tt.role)
		})

	}
}

func TestCertRotationRbacTestSuite(t *testing.T) {
	logrus.Infof("Beginning testing")
	suite.Run(t, new(CertRotationRbacTestSuite))
}
