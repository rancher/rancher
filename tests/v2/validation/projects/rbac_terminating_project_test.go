//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	projectsApi "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	project "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rancherleader"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	pod "github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
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

func (rtp *RbacTerminatingProjectTestSuite) TearDownSuite() {
	rtp.session.Cleanup()
}

func (rtp *RbacTerminatingProjectTestSuite) SetupSuite() {
	rtp.session = session.NewSession()

	client, err := rancher.NewClient("", rtp.session)
	require.NoError(rtp.T(), err)

	rtp.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rtp.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rtp.client, clusterName)
	require.NoError(rtp.T(), err, "Error getting cluster ID")
	rtp.cluster, err = rtp.client.Management.Cluster.ByID(clusterID)
	require.NoError(rtp.T(), err)
}

func (rtp *RbacTerminatingProjectTestSuite) TestUserAdditionToClusterWithTerminatingProjectNamespace() {
	subSession := rtp.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(rtp.client, users.UserConfig(), projectsApi.StandardUser)
	require.NoError(rtp.T(), err)
	rtp.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	projectTemplate := projects.NewProjectTemplate(rtp.cluster.ID)
	createdProject, err := rtp.client.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(rtp.T(), err)
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(rtp.T(), err)

	logCaptureStartTime := time.Now()
	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	updatedProject, err := project.UpdateProjectNamespaceFinalizer(rtp.client, createdProject, finalizer)
	require.NoError(rtp.T(), err, "Failed to update finalizer.")
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 3)
	require.NoError(rtp.T(), err)

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(rtp.client, createdProject.Namespace, createdProject.Name)
	require.Error(rtp.T(), err)
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 1)
	require.NoError(rtp.T(), err)
	leaderPodName, err := rancherleader.GetRancherLeaderPodName(rtp.client)
	require.NoError(rtp.T(), err)
	errorRegex := `\[INFO\] \[mgmt-project-rbac-remove\] Deleting namespace ` + regexp.QuoteMeta(createdProject.Name)
	err = pod.CheckPodLogsForErrors(rtp.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.Error(rtp.T(), err)

	logCaptureStartTime = time.Now()
	log.Info("Add the standard user to the downstream cluster as cluster owner.")
	err = users.AddClusterRoleToUser(rtp.client, rtp.cluster, createdUser, rbac.ClusterOwner.String(), nil)
	require.NoError(rtp.T(), err)

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	errorRegex = `\[ERROR\] error syncing '(.*?)': handler mgmt-auth-crtb-controller: .*? (?:not found|is forbidden), requeuing`
	err = pod.CheckPodLogsForErrors(rtp.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(rtp.T(), err)

	logCaptureStartTime = time.Now()
	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = project.UpdateProjectNamespaceFinalizer(rtp.client, updatedProject, finalizer)
	require.NoError(rtp.T(), err, "Failed to remove the finalizer.")

	log.Info("Verify that there are no errors in the Rancher logs related to role binding.")
	err = pod.CheckPodLogsForErrors(rtp.client, projectsApi.LocalCluster, leaderPodName, projectsApi.RancherNamespace, errorRegex, logCaptureStartTime)
	require.NoError(rtp.T(), err)
}

func (rtp *RbacTerminatingProjectTestSuite) TestUserAdditionToProjectWithTerminatingProjectNamespace() {
	subSession := rtp.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user.")
	createdUser, err := users.CreateUserWithRole(rtp.client, users.UserConfig(), projectsApi.StandardUser)
	require.NoError(rtp.T(), err)
	rtp.T().Logf("Created user: %v", createdUser.Username)

	log.Info("Create a project in the downstream cluster.")
	projectTemplate := projects.NewProjectTemplate(rtp.cluster.ID)
	createdProject, err := rtp.client.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(rtp.T(), err)
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(rtp.T(), err)

	log.Info("Simulate a project stuck in terminating state by adding a finalizer to the project.")
	finalizer := append([]string{dummyFinalizer}, createdProject.Finalizers...)
	updatedProject, err := project.UpdateProjectNamespaceFinalizer(rtp.client, createdProject, finalizer)
	require.NoError(rtp.T(), err, "Failed to update finalizer.")
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 3)
	require.NoError(rtp.T(), err)

	log.Info("Delete the Project.")
	err = projectsApi.DeleteProject(rtp.client, createdProject.Namespace, createdProject.Name)
	require.Error(rtp.T(), err)
	err = project.WaitForProjectFinalizerToUpdate(rtp.client, createdProject.Name, createdProject.Namespace, 1)
	require.NoError(rtp.T(), err)

	log.Info("Add the standard user to the project as project owner.")
	_, err = createProjectRoleTemplateBinding(rtp.client, createdUser, createdProject, rbac.ProjectOwner.String())
	require.Error(rtp.T(), err)
	prtbNamePlaceholder := `[^"]+`
	regexPattern := fmt.Sprintf(`projectroletemplatebindings\.management\.cattle\.io "%s" is forbidden: unable to create new content in namespace %s because it is being terminated`, prtbNamePlaceholder, regexp.QuoteMeta(createdProject.Name))
	expectedErrorMessage := regexp.MustCompile(regexPattern)
	require.Regexp(rtp.T(), expectedErrorMessage, err.Error())

	log.Info("Remove the finalizer that was previously added to the project.")
	finalizer = nil
	_, err = project.UpdateProjectNamespaceFinalizer(rtp.client, updatedProject, finalizer)
	require.NoError(rtp.T(), err, "Failed to remove the finalizer.")
}

func TestRbacTerminatingProjectTestSuite(t *testing.T) {
	suite.Run(t, new(RbacTerminatingProjectTestSuite))
}
