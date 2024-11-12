//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	project "github.com/rancher/rancher/tests/v2/actions/projects"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
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
	projectTemplate := projects.NewProjectTemplate(projects.LocalCluster)
	createdProject, err := pr.client.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(pr.T(), err, "Failed to create project")
	err = project.WaitForProjectFinalizerToUpdate(pr.client, createdProject.Name, createdProject.Namespace, 2)
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
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, rbac.ClusterOwner.String(), nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project in the downstream cluster and verify that the project can be listed.")
	projectTemplate := projects.NewProjectTemplate(pr.cluster.ID)
	createdProject, err := standardUserClient.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(pr.T(), err, "Failed to create project")
	err = project.WaitForProjectFinalizerToUpdate(standardUserClient, createdProject.Name, createdProject.Namespace, 2)
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

func (pr *ProjectsTestSuite) TestMoveNamespaceOutOfProject() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, rbac.ClusterOwner.String(), nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project in the downstream cluster and a namespace in the project.")
	projectTemplate := projects.NewProjectTemplate(pr.cluster.ID)
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, pr.cluster.ID, projectTemplate)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, pr.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(pr.T(), err)

	log.Info("Move the namespace out of the project.")
	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, pr.cluster.ID, createdNamespace.Name)
	require.NoError(pr.T(), err)
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
	err = project.WaitForProjectIDUpdate(standardUserClient, pr.cluster.ID, createdProject.Name, updatedNamespace.Name)
	require.Error(pr.T(), err)
}

func (pr *ProjectsTestSuite) TestProjectWithResourceQuotaAndContainerDefaultResourceLimit() {
	subSession := pr.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(pr.client, users.UserConfig(), projects.StandardUser)
	require.NoError(pr.T(), err, "Failed to create standard user")
	standardUserClient, err := pr.client.AsUser(standardUser)
	require.NoError(pr.T(), err)
	err = users.AddClusterRoleToUser(pr.client, pr.cluster, standardUser, rbac.ClusterOwner.String(), nil)
	require.NoError(pr.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	log.Info("Create a project (with resource quota and container default resource limit) and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	cpuLimit := "100m"
	cpuReservation := "50m"
	memoryLimit := "64Mi"
	memoryReservation := "32Mi"
	projectTemplate := projects.NewProjectTemplate(pr.cluster.ID)
	projectTemplate.Spec.ContainerDefaultResourceLimit.LimitsCPU = cpuLimit
	projectTemplate.Spec.ContainerDefaultResourceLimit.RequestsCPU = cpuReservation
	projectTemplate.Spec.ContainerDefaultResourceLimit.LimitsMemory = memoryLimit
	projectTemplate.Spec.ContainerDefaultResourceLimit.RequestsMemory = memoryReservation
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	projectTemplate.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, pr.cluster.ID, projectTemplate)
	require.NoError(pr.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, pr.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(pr.T(), err)

	log.Info("Verify that the pod limits and container default resource limit in the Project spec is accurate.")
	projectSpec := createdProject.Spec
	require.Equal(pr.T(), namespacePodLimit, projectSpec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(pr.T(), projectPodLimit, projectSpec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")
	require.Equal(pr.T(), cpuLimit, projectSpec.ContainerDefaultResourceLimit.LimitsCPU, "CPU limit mismatch")
	require.Equal(pr.T(), cpuReservation, projectSpec.ContainerDefaultResourceLimit.RequestsCPU, "CPU reservation mismatch")
	require.Equal(pr.T(), memoryLimit, projectSpec.ContainerDefaultResourceLimit.LimitsMemory, "Memory limit mismatch")
	require.Equal(pr.T(), memoryReservation, projectSpec.ContainerDefaultResourceLimit.RequestsMemory, "Memory reservation mismatch")

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, pr.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(pr.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, pr.cluster.ID, createdNamespace.Name, namespacePodLimit, true, "")
	require.NoError(pr.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, pr.cluster.ID, createdNamespace.Name, 2)
	require.NoError(pr.T(), err)

	log.Info("Verify that the limit range object is created for the namespace and the resource limit in the limit range is accurate.")
	err = checkLimitRange(standardUserClient, pr.cluster.ID, createdNamespace.Name, cpuLimit, cpuReservation, memoryLimit, memoryReservation)
	require.NoError(pr.T(), err)

	log.Info("Create a deployment in the namespace with two replicas and verify that the pods are created.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, pr.cluster.ID, createdNamespace.Name, 2, "", "", false, false, false, true)
	require.NoError(pr.T(), err, "Failed to create deployment in the namespace")

	log.Info("Verify that the resource limits and requests for the container in the pod spec is accurate.")
	err = checkContainerResources(standardUserClient, pr.cluster.ID, createdNamespace.Name, createdDeployment.Name, cpuLimit, cpuReservation, memoryLimit, memoryReservation)
	require.NoError(pr.T(), err)
}

func TestProjectsTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectsTestSuite))
}
