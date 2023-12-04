//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	projectsApi "github.com/rancher/shepherd/extensions/kubeapi/projects"
	"github.com/rancher/shepherd/extensions/rancherleader"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RbacTerminatingProjectTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (pr *RbacTerminatingProjectTestSuite) TearDownSuite() {
	pr.session.Cleanup()
}

func (pr *RbacTerminatingProjectTestSuite) SetupSuite() {
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

func (pr *RbacTerminatingProjectTestSuite) TestUserAdditionToClusterWithTerminatingProjectNamespace() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projectsApi.StandardUser)
	require.NoError(pr.T(), err)
	pr.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	createdProject, err := createProject(pr.client, pr.cluster.ID)
	require.NoError(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(pr.T(), err)

	logCaptureStartTime := time.Now()
	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	updatedProject, err := updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 3)
	require.NoError(pr.T(), err)

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(pr.client, createdProject.Namespace, createdProject.Name)
	require.Error(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 1)
	require.NoError(pr.T(), err)
	leaderPodName, err := rancherleader.GetRancherLeaderPodName(pr.client)
	require.NoError(pr.T(), err)
	errorRegex := `\[INFO\] \[mgmt-project-rbac-remove\] Deleting namespace ` + regexp.QuoteMeta(createdProject.Name)
	err = checkPodLogsForErrors(pr.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.Error(pr.T(), err)

	logCaptureStartTime = time.Now()
	log.Info("Add the standard user to the downstream cluster as cluster owner.")
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, createdUser, roleOwner, nil)
	require.NoError(pr.T(), err)

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	errorRegex = `\[ERROR\] error syncing '(.*?)': handler mgmt-auth-crtb-controller: .*? (?:not found|is forbidden), requeuing`
	err = checkPodLogsForErrors(pr.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(pr.T(), err)

	logCaptureStartTime = time.Now()
	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = updateProjectNamespaceFinalizer(pr.client, updatedProject, finalizer)
	require.NoError(pr.T(), err, "Failed to remove the finalizer.")

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	err = checkPodLogsForErrors(pr.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(pr.T(), err)
}

func (pr *RbacTerminatingProjectTestSuite) TestUserAdditionToProjectWithTerminatingProjectNamespace() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projectsApi.StandardUser)
	require.NoError(pr.T(), err)
	pr.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	createdProject, err := createProject(pr.client, pr.cluster.ID)
	require.NoError(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(pr.T(), err)

	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	updatedProject, err := updateProjectNamespaceFinalizer(pr.client, createdProject, finalizer)
	require.NoError(pr.T(), err, "Failed to update finalizer.")
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 3)
	require.NoError(pr.T(), err)

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(pr.client, createdProject.Namespace, createdProject.Name)
	require.Error(pr.T(), err)
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 1)
	require.NoError(pr.T(), err)

	log.Info("Add the standard user to the project as project owner.")
	_, err = createProjectRoleTemplateBinding(pr.client, createdUser, createdProject, roleProjectOwner)
	require.Error(pr.T(), err)
	prtbNamePlaceholder := `[^"]+`
	regexPattern := fmt.Sprintf(`projectroletemplatebindings\.management\.cattle\.io "%s" is forbidden: unable to create new content in namespace %s because it is being terminated`, prtbNamePlaceholder, regexp.QuoteMeta(createdProject.Name))
	expectedErrorMessage := regexp.MustCompile(regexPattern)
	require.Regexp(pr.T(), expectedErrorMessage, err.Error())

	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = updateProjectNamespaceFinalizer(pr.client, updatedProject, finalizer)
	require.NoError(pr.T(), err, "Failed to remove the finalizer.")
}

func TestRbacTerminatingProjectTestSuite(t *testing.T) {
	suite.Run(t, new(RbacTerminatingProjectTestSuite))
}
