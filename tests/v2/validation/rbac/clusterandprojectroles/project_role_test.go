//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package clusterandprojectroles

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
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

type ProjectRolesTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (pr *ProjectRolesTestSuite) TearDownSuite() {
	pr.session.Cleanup()
}

func (pr *ProjectRolesTestSuite) SetupSuite() {
	pr.session = session.NewSession()
	client, err := rancher.NewClient("", pr.session)
	require.NoError(pr.T(), err)

	pr.client = client

	log.Info("Getting cluster name from the config file and append cluster details in pr")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(pr.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(pr.client, clusterName)
	require.NoError(pr.T(), err, "Error getting cluster ID")
	pr.cluster, err = pr.client.Management.Cluster.ByID(clusterID)
	require.NoError(pr.T(), err)
}

func (pr *ProjectRolesTestSuite) testSetupUserAndProject() (*rancher.Client, *management.Project) {
	pr.T().Log("Set up User with cluster Role for additional rbac test cases " + rbac.ClusterOwner)
	newUser, standardUserClient, err := rbac.SetupUser(pr.client, rbac.StandardUser.String())
	require.NoError(pr.T(), err)

	createProjectAsAdmin, err := pr.client.Management.Project.Create(projects.NewProjectConfig(pr.cluster.ID))
	require.NoError(pr.T(), err)

	log.Info("Adding a standard user as project Owner in the admin project")
	errUserRole := users.AddProjectMember(pr.client, createProjectAsAdmin, newUser, rbac.ProjectOwner.String(), nil)
	require.NoError(pr.T(), errUserRole)
	standardUserClient, err = standardUserClient.ReLogin()
	require.NoError(pr.T(), err)
	return standardUserClient, createProjectAsAdmin
}

func (pr *ProjectRolesTestSuite) TestProjectOwnerAddsAndRemovesOtherProjectOwners() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, adminProject := pr.testSetupUserAndProject()
	additionalUser, additionalUserClient, err := rbac.SetupUser(pr.client, rbac.StandardUser.String())
	require.NoError(pr.T(), err)

	errUserRole := users.AddProjectMember(standardUserClient, adminProject, additionalUser, rbac.ProjectOwner.String(), nil)
	require.NoError(pr.T(), errUserRole)
	additionalUserClient, err = additionalUserClient.ReLogin()
	require.NoError(pr.T(), err)

	userGetProject, err := projects.ListProjectNames(additionalUserClient, pr.cluster.ID)
	require.NoError(pr.T(), err)
	assert.Equal(pr.T(), 1, len(userGetProject))
	assert.Equal(pr.T(), adminProject.Name, userGetProject[0])

	errRemoveMember := users.RemoveProjectMember(standardUserClient, additionalUser)
	require.NoError(pr.T(), errRemoveMember)

	userProjectEmptyAfterRemoval, err := projects.ListProjectNames(additionalUserClient, pr.cluster.ID)
	require.Empty(pr.T(), userProjectEmptyAfterRemoval)
}

func (pr *ProjectRolesTestSuite) TestManageProjectUserRoleCannotAddProjectOwner() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, adminProject := pr.testSetupUserAndProject()
	additionalUser, additionalUserClient, err := rbac.SetupUser(pr.client, rbac.StandardUser.String())
	require.NoError(pr.T(), err)

	errUserRole := users.AddProjectMember(standardUserClient, adminProject, additionalUser, rbac.CustomManageProjectMember.String(), nil)
	require.NoError(pr.T(), errUserRole)
	additionalUserClient, err = additionalUserClient.ReLogin()
	require.NoError(pr.T(), err)

	addNewUserAsProjectOwner, addNewUserAsPOClient, err := rbac.SetupUser(pr.client, rbac.StandardUser.String())
	require.NoError(pr.T(), err)

	errUserRole2 := users.AddProjectMember(additionalUserClient, adminProject, addNewUserAsProjectOwner, rbac.ProjectOwner.String(), nil)
	require.Error(pr.T(), errUserRole2)
	assert.Contains(pr.T(), errUserRole2.Error(), "422 Unprocessable Entity")

	addNewUserAsPOClient, err = addNewUserAsPOClient.ReLogin()
	require.NoError(pr.T(), err)

	userGetProject, err := projects.ListProjectNames(addNewUserAsPOClient, pr.cluster.ID)
	require.NoError(pr.T(), err)
	assert.Equal(pr.T(), 0, len(userGetProject))
}

func TestProjectRolesTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectRolesTestSuite))
}
