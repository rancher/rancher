//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"regexp"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/constants"
	projectsApi "github.com/rancher/rancher/tests/framework/extensions/kubeapi/projects"
	"github.com/rancher/rancher/tests/framework/extensions/rancherleader"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProjectsAdditionalTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (pr *ProjectsAdditionalTestSuite) TearDownSuite() {
	pr.session.Cleanup()
}

func (pr *ProjectsAdditionalTestSuite) SetupSuite() {
	pr.session = session.NewSession()

	client, err := rancher.NewClient("", pr.session)
	require.NoError(pr.T(), err)

	pr.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(pr.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(pr.client, clusterName)
	require.NoError(pr.T(), err, "Error getting cluster ID")
	pr.cluster, err = pr.client.Management.Cluster.ByID(clusterID)
	require.NoError(pr.T(), err)

}

func (pr *ProjectsAdditionalTestSuite) TestUserAdditionToClusterWithTerminatingProjectNamespace() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), constants.StandardUser)
	require.NoError(pr.T(), err)
	pr.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	createdProject, err := createProject(pr.client, pr.cluster.ID)
	require.NoError(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name)
	require.NoError(pr.T(), err)

	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	_, err = updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")

	logCaptureStartTime := time.Now()

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(pr.client, createdProject)
	require.NoError(pr.T(), err)

	log.Info("Add the standard user to the downstream cluster as cluster owner.")
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, createdUser, constants.RoleOwner, nil)
	require.NoError(pr.T(), err)

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	leaderPodName, err := rancherleader.GetRancherLeaderPod(pr.client)
	require.NoError(pr.T(), err)
	errorRegex := `\[ERROR\] error syncing '(.*?)': handler mgmt-auth-crtb-controller: .*? (?:not found|is forbidden), requeuing`
	err = checkPodLogsForErrors(pr.client, constants.LocalCluster, leaderPodName, constants.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(pr.T(), err)

	logCaptureStartTime = time.Now()

	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	err = checkPodLogsForErrors(pr.client, constants.LocalCluster, leaderPodName, constants.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(pr.T(), err)
}

func (pr *ProjectsAdditionalTestSuite) TestUserAdditionToProjectWithTerminatingProjectNamespace() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), constants.StandardUser)
	require.NoError(pr.T(), err)
	pr.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	createdProject, err := createProject(pr.client, pr.cluster.ID)
	require.NoError(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name)
	require.NoError(pr.T(), err)

	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	_, err = updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(pr.client, createdProject)
	require.NoError(pr.T(), err)

	log.Info("Add the standard user to the project as project owner.")
	_, err = createProjectRoleTemplateBinding(pr.client, createdUser, createdProject, constants.RoleProjectOwner)
	require.Error(pr.T(), err)
	pattern := regexp.MustCompile(`projectroletemplatebindings\.management\.cattle\.io ".+?" is forbidden: unable to create new content in namespace ` + regexp.QuoteMeta(createdProject.Name) + ` because it is being terminated`)
	require.Regexp(pr.T(), pattern, err.Error())

	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")
}

func TestProjectsAdditionalTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectsAdditionalTestSuite))
}
