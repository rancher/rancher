//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"fmt"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/kubeapi/namespaces"
	"github.com/rancher/shepherd/extensions/kubeapi/projects"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (pr *ProjectsTestSuite) TearDownSuite() {
	pr.session.Cleanup()
}

func (pr *ProjectsTestSuite) SetupSuite() {
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

func (pr *ProjectsTestSuite) TestProjectsCrudLocalCluster() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a project in the local cluster and verify that the project can be listed.")
	projectTemplate := NewProjectTemplate(projects.LocalCluster)
	createdProject, err := createProject(pr.client, projectTemplate)
	require.NoError(pr.T(), err, "Failed to create project")
	err = waitForFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(pr.T(), err)
	projectList, err := projects.ListProjects(pr.client, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 1, len(projectList.Items), "Expected exactly one project.")

	log.Info("Verify that the project can be updated by adding a label.")
	currentProject := projectList.Items[0]
	if currentProject.Labels == nil {
		currentProject.Labels = make(map[string]string)
	}
	currentProject.Labels["hello"] = "world"
	_, err = pr.client.WranglerContext.Mgmt.Project().Update(&currentProject)
	require.NoError(pr.T(), err, "Failed to update project.")

	updatedProjectList, err := projects.ListProjects(pr.client, createdProject.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "hello", "world"),
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 1, len(updatedProjectList.Items), "Expected one project in the list")

	log.Info("Delete the project.")
	err = projects.DeleteProject(pr.client, createdProject.Namespace, createdProject.Name)
	require.NoError(pr.T(), err, "Failed to delete project")
	projectList, err = projects.ListProjects(pr.client, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 0, len(projectList.Items), "Expected zero project.")
}

func (pr *ProjectsTestSuite) TestProjectsCrudDownstreamCluster() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, clusterOwner, nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project in the downstream cluster and verify that the project can be listed.")
	projectTemplate := NewProjectTemplate(pr.cluster.ID)
	createdProject, err := createProject(standardUserClient, projectTemplate)
	require.NoError(pr.T(), err, "Failed to create project")
	err = waitForFinalizerToUpdate(standardUserClient, createdProject.Name, createdProject.Namespace, 2)
	require.NoError(pr.T(), err)
	projectList, err := projects.ListProjects(standardUserClient, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(pr.T(), err, "Failed to list project.")
	require.Equal(pr.T(), 1, len(projectList.Items), "Expected exactly one project.")

	log.Info("Verify that the project can be updated by adding a label.")
	currentProject := projectList.Items[0]
	if currentProject.Labels == nil {
		currentProject.Labels = make(map[string]string)
	}
	currentProject.Labels["hello"] = "world"
	_, err = standardUserClient.WranglerContext.Mgmt.Project().Update(&currentProject)
	require.NoError(pr.T(), err, "Failed to update project.")

	updatedProjectList, err := projects.ListProjects(standardUserClient, createdProject.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "hello", "world"),
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 1, len(updatedProjectList.Items), "Expected one project in the list")

	log.Info("Delete the project.")
	err = projects.DeleteProject(standardUserClient, createdProject.Namespace, createdProject.Name)
	require.NoError(pr.T(), err, "Failed to delete project")
	projectList, err = projects.ListProjects(standardUserClient, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(pr.T(), err, "Failed to list project.")
	require.Equal(pr.T(), 0, len(projectList.Items), "Expected zero project.")
}

func (pr *ProjectsTestSuite) TestDeleteSystemProject() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Delete the System Project in the local cluster.")
	projectList, err := projects.ListProjects(pr.client, projects.LocalCluster, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", systemProjectLabel, "true"),
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 1, len(projectList.Items), "Expected one project in the list")

	systemProjectName := projectList.Items[0].ObjectMeta.Name
	err = projects.DeleteProject(pr.client, projects.LocalCluster, systemProjectName)
	require.Error(pr.T(), err, "Failed to delete project")
	expectedErrorMessage := "admission webhook \"rancher.cattle.io.projects.management.cattle.io\" denied the request: System Project cannot be deleted"
	require.Equal(pr.T(), expectedErrorMessage, err.Error())

	log.Info("Delete the System Project in the downstream cluster.")
	projectList, err = projects.ListProjects(pr.client, pr.cluster.ID, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", systemProjectLabel, "true"),
	})
	require.NoError(pr.T(), err)
	require.Equal(pr.T(), 1, len(projectList.Items), "Expected one project in the list")

	systemProjectName = projectList.Items[0].ObjectMeta.Name
	err = projects.DeleteProject(pr.client, pr.cluster.ID, systemProjectName)
	require.Error(pr.T(), err, "Failed to delete project")
	expectedErrorMessage = "admission webhook \"rancher.cattle.io.projects.management.cattle.io\" denied the request: System Project cannot be deleted"
	require.Equal(pr.T(), expectedErrorMessage, err.Error())
}

func (pr *ProjectsTestSuite) TestProjectWithoutResourceQuota() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, clusterOwner, nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project (without any resource quota) and a namespace in the project.")
	projectTemplate := NewProjectTemplate(pr.cluster.ID)
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, pr.cluster.ID, projectTemplate)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, createdNamespace.Name)
	require.NoError(pr.T(), err)
	err = checkNamespaceLabelsAndAnnotations(pr.cluster.ID, createdProject.Name, updatedNamespace)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, pr.cluster.ID, updatedNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(pr.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Create a deployment in the namespace with ten replicas.")
	deployment, err := createDeployment(standardUserClient, pr.cluster.ID, updatedNamespace.Name, 10)
	require.NoError(pr.T(), err, "Failed to create deployment in the namespace")

	log.Info("Verify that there are ten pods created in the deployment and they are in Running state.")
	err = charts.WatchAndWaitDeployments(standardUserClient, pr.cluster.ID, updatedNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	require.NoError(pr.T(), err)
}

func (pr *ProjectsTestSuite) TestMoveNamespaceOutOfProject() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, clusterOwner, nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project in the downstream cluster and a namespace in the project.")
	projectTemplate := NewProjectTemplate(pr.cluster.ID)
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, pr.cluster.ID, projectTemplate)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, createdNamespace.Name)
	require.NoError(pr.T(), err)
	err = checkNamespaceLabelsAndAnnotations(pr.cluster.ID, createdProject.Name, updatedNamespace)
	require.NoError(pr.T(), err)

	log.Info("Move the namespace out of the project.")
	delete(updatedNamespace.Labels, projects.ProjectIDAnnotation)
	delete(updatedNamespace.Annotations, projects.ProjectIDAnnotation)

	downstreamContext, err := pr.client.WranglerContext.DownStreamClusterWranglerContext(pr.cluster.ID)
	require.NoError(pr.T(), err)

	currentNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, updatedNamespace.Name)
	require.NoError(pr.T(), err)
	updatedNamespace.ResourceVersion = currentNamespace.ResourceVersion
	_, err = downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(pr.T(), err, "Failed to move the namespace out of the project")

	log.Info("Verify that the namespace does not have the label and annotation referencing the project.")
	movedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, updatedNamespace.Name)
	require.NoError(pr.T(), err)
	err = checkNamespaceLabelsAndAnnotations(pr.cluster.ID, createdProject.Name, movedNamespace)
	require.Error(pr.T(), err)
}

func (pr *ProjectsTestSuite) TestMoveNamespaceBetweenProjectsWithNoResourceQuota() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, clusterOwner, nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project in the downstream cluster and a namespace in the project.")
	projectTemplate := NewProjectTemplate(pr.cluster.ID)
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = ""
	projectTemplate.Spec.ResourceQuota.Limit.Pods = ""
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, pr.cluster.ID, projectTemplate)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, createdNamespace.Name)
	require.NoError(pr.T(), err)
	err = checkNamespaceLabelsAndAnnotations(pr.cluster.ID, createdProject.Name, updatedNamespace)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, pr.cluster.ID, updatedNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(pr.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Create a deployment in the namespace with ten replicas.")
	deployment, err := createDeployment(standardUserClient, createdProject.Namespace, updatedNamespace.Name, 10)
	require.NoError(pr.T(), err, "Failed to create deployment in the namespace")

	log.Info("Verify that there are ten pods created in the deployment and they are in Running state.")
	err = charts.WatchAndWaitDeployments(standardUserClient, createdProject.Namespace, updatedNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	require.NoError(pr.T(), err)

	log.Info("Create another project in the downstream cluster.")
	projectTemplate = NewProjectTemplate(pr.cluster.ID)
	createdProject2, err := createProject(standardUserClient, projectTemplate)
	require.NoError(pr.T(), err, "Failed to create project")
	err = waitForFinalizerToUpdate(pr.client, createdProject2.Name, createdProject2.Namespace, 2)
	require.NoError(pr.T(), err)

	log.Info("Move the namespace from the first project to the second project.")
	currentNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, updatedNamespace.Name)
	require.NoError(pr.T(), err)
	downstreamContext, err := pr.client.WranglerContext.DownStreamClusterWranglerContext(pr.cluster.ID)
	require.NoError(pr.T(), err)

	updatedNamespace.Annotations[projects.ProjectIDAnnotation] = createdProject2.Namespace + ":" + createdProject2.Name
	updatedNamespace.ResourceVersion = currentNamespace.ResourceVersion
	_, err = downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the correct label and annotation referencing the second project.")
	movedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, updatedNamespace.Name)
	require.NoError(pr.T(), err)
	err = checkNamespaceLabelsAndAnnotations(pr.cluster.ID, createdProject2.Name, movedNamespace)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, pr.cluster.ID, updatedNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(pr.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Verify that the deployment is in Active state and all pods in the deployment are in Running state.")
	err = charts.WatchAndWaitDeployments(standardUserClient, pr.cluster.ID, movedNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	require.NoError(pr.T(), err)
}

func TestProjectsTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectsTestSuite))
}
