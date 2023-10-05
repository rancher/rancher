package rbac

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ETCDRbacBackupTestSuite struct {
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

func (rb *ETCDRbacBackupTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *ETCDRbacBackupTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rb.session = testSession

	rb.namespace = defaultNamespace

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client
	rb.clusterName = client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), rb.clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, rb.clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)
}

func (rb *ETCDRbacBackupTestSuite) ValidateEtcdSnapshotCluster(role string) {

	log.Infof("Creating a snapshot of the cluster as %v", role)

	err := etcdsnapshot.CreateSnapshot(rb.standardUserClient, rb.clusterName)
	switch role {
	case roleOwner, restrictedAdmin:
		require.NoError(rb.T(), err)

	case roleMember, roleProjectOwner, roleProjectMember:
		require.Error(rb.T(), err)
		assert.Equal(rb.T(), "Resource type [provisioning.cattle.io.cluster] is not updatable", err.Error())
	}
}

func (rb *ETCDRbacBackupTestSuite) TestETCDRbac() {
	clusterID, err := clusters.GetClusterIDByName(rb.client, rb.clusterName)
	require.NoError(rb.T(), err)
	if !(strings.Contains(clusterID, "c-m-")) {
		rb.T().Skip("Skipping tests since cluster is not of type - k3s or RKE2")
	}
	tests := []struct {
		name   string
		role   string
		member string
	}{
		{"Cluster Owner", roleOwner, standardUser},
		{"Cluster Member", roleMember, standardUser},
		{"Project Owner", roleProjectOwner, standardUser},
		{"Project Member", roleProjectMember, standardUser},
		{"Restricted Admin", restrictedAdmin, restrictedAdmin},
	}
	for _, tt := range tests {
		rb.Run("Set up User with Cluster Role "+tt.name, func() {
			newUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), tt.member)
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
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.standardUser, tt.role, nil)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.standardUser, tt.role, nil)
					require.NoError(rb.T(), err)
				}
			}

			relogin, err := rb.standardUserClient.ReLogin()
			require.NoError(rb.T(), err)
			rb.standardUserClient = relogin
		})

		rb.T().Logf("Starting validations for %v", tt.role)

		rb.Run("Test case - Take Etcd snapshot of a cluster as a "+tt.name, func() {
			rb.ValidateEtcdSnapshotCluster(tt.role)
		})

	}
}

func TestETCDRbacBackupTestSuite(t *testing.T) {
	suite.Run(t, new(ETCDRbacBackupTestSuite))
}
