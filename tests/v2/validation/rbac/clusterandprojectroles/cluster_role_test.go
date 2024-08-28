//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package clusterandprojectroles

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClusterRoleTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (rb *ClusterRoleTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *ClusterRoleTestSuite) SetupSuite() {
	rb.session = session.NewSession()

	client, err := rancher.NewClient("", rb.session)
	require.NoError(rb.T(), err)

	rb.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rb")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)
}

func (rb *ClusterRoleTestSuite) TestClusterOwnerAddsUserAsProjectOwner() {
	subSession := rb.session.NewSession()
	defer subSession.Cleanup()

	clusterAdmin, clusterAdminClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	additionalUser, additionalUserClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	rb.T().Logf("Adding user as " + rbac.ClusterOwner.String() + " to the downstream cluster.")
	err = users.AddClusterRoleToUser(rb.client, rb.cluster, clusterAdmin, rbac.ClusterOwner.String(), nil)
	require.NoError(rb.T(), err)
	clusterAdminClient, err = clusterAdminClient.ReLogin()
	require.NoError(rb.T(), err)

	clusterOwnerProject, err := clusterAdminClient.Management.Project.Create(projects.NewProjectConfig(rb.cluster.ID))
	require.NoError(rb.T(), err)
	err = users.AddProjectMember(clusterAdminClient, clusterOwnerProject, additionalUser, rbac.ProjectOwner.String(), nil)
	require.NoError(rb.T(), err)

	projectListClusterAdditionalUser, err := projects.ListProjectNames(additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(projectListClusterAdditionalUser))
	assert.Equal(rb.T(), clusterOwnerProject.Name, projectListClusterAdditionalUser[0])

	err = users.RemoveProjectMember(clusterAdminClient, additionalUser)
	require.NoError(rb.T(), err)
	projectListClusterAdditionalUser, err = projects.ListProjectNames(additionalUserClient, rb.cluster.ID)
	require.Empty(rb.T(), projectListClusterAdditionalUser)
}

func (rb *ClusterRoleTestSuite) TestClusterOwnerAddsUserAsClusterOwner() {
	subSession := rb.session.NewSession()
	defer subSession.Cleanup()

	clusterAdmin, clusterAdminClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	err = users.AddClusterRoleToUser(rb.client, rb.cluster, clusterAdmin, rbac.ClusterOwner.String(), nil)
	require.NoError(rb.T(), err)
	clusterAdminClient, err = clusterAdminClient.ReLogin()
	require.NoError(rb.T(), err)

	additionalUser, additionalUserClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	err = users.AddClusterRoleToUser(clusterAdminClient, rb.cluster, additionalUser, rbac.ClusterOwner.String(), nil)
	require.NoError(rb.T(), err)
	additionalUserClient, err = additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	clusterList, err := additionalUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	err = users.RemoveClusterRoleFromUser(clusterAdminClient, additionalUser)
	require.NoError(rb.T(), err)
	clusterList, err = additionalUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.Empty(rb.T(), clusterList.Data)
}

func (rb *ClusterRoleTestSuite) TestClusterOwnerAddsClusterMemberAsProjectOwner() {
	subSession := rb.session.NewSession()
	defer subSession.Cleanup()

	clusterAdmin, clusterAdminClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	err = users.AddClusterRoleToUser(rb.client, rb.cluster, clusterAdmin, rbac.ClusterOwner.String(), nil)
	require.NoError(rb.T(), err)
	clusterAdminClient, err = clusterAdminClient.ReLogin()
	require.NoError(rb.T(), err)

	additionalUser, additionalUserClient, err := rbac.SetupUser(rb.client, rbac.StandardUser.String())
	require.NoError(rb.T(), err)

	err = users.AddClusterRoleToUser(clusterAdminClient, rb.cluster, additionalUser, rbac.ClusterMember.String(), nil)
	require.NoError(rb.T(), err)
	additionalUserClient, err = additionalUserClient.ReLogin()
	require.NoError(rb.T(), err)

	clusterList, err := additionalUserClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ListAll(nil)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), 1, len(clusterList.Data))

	clusterOwnerProject, err := clusterAdminClient.Management.Project.Create(projects.NewProjectConfig(rb.cluster.ID))
	require.NoError(rb.T(), err)

	err = users.AddProjectMember(clusterAdminClient, clusterOwnerProject, additionalUser, rbac.ProjectOwner.String(), nil)
	require.NoError(rb.T(), err)
	projectListProjectOwner, err := projects.ListProjectNames(additionalUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)

	assert.Equal(rb.T(), 1, len(projectListProjectOwner))
	assert.Equal(rb.T(), clusterOwnerProject.Name, projectListProjectOwner[0])
}

func TestClusterRoleTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterRoleTestSuite))
}
