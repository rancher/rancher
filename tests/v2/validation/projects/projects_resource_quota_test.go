//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	quotas "github.com/rancher/rancher/tests/v2/actions/kubeapi/resourcequotas"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	project "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wrangler"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type ProjectsResourceQuotaTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (prq *ProjectsResourceQuotaTestSuite) TearDownSuite() {
	prq.session.Cleanup()
}

func (prq *ProjectsResourceQuotaTestSuite) SetupSuite() {
	prq.session = session.NewSession()

	client, err := rancher.NewClient("", prq.session)
	require.NoError(prq.T(), err)

	prq.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(prq.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(prq.client, clusterName)
	require.NoError(prq.T(), err, "Error getting cluster ID")
	prq.cluster, err = prq.client.Management.Cluster.ByID(clusterID)
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) setupUserForProject() (*rancher.Client, *wrangler.Context) {
	log.Info("Create a standard user and add the user to the downstream cluster as cluster owner.")
	standardUser, err := users.CreateUserWithRole(prq.client, users.UserConfig(), projects.StandardUser)
	require.NoError(prq.T(), err, "Failed to create standard user")
	standardUserClient, err := prq.client.AsUser(standardUser)
	require.NoError(prq.T(), err)
	err = users.AddClusterRoleToUser(prq.client, prq.cluster, standardUser, rbac.ClusterOwner.String(), nil)
	require.NoError(prq.T(), err, "Failed to add the user as a cluster owner to the downstream cluster")

	standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)

	return standardUserClient, standardUserContext
}

func (prq *ProjectsResourceQuotaTestSuite) TestProjectWithoutResourceQuota() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (without any resource quota) and a namespace in the project.")
	projectTemplate := projects.NewProjectTemplate(prq.cluster.ID)
	createdProject, createdNamespace, err := createProjectAndNamespace(standardUserClient, prq.cluster.ID, projectTemplate)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Create a deployment in the namespace with ten replicas.")
	_, err = deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 10, "", "", false, false, false, true)
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestProjectWithResourceQuota() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quotas) and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, firstNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the pod limits in the Project spec is accurate.")
	require.Equal(prq.T(), namespacePodLimit, createdProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, createdProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, firstNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, firstNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, firstNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, firstNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Create another namespace in the project and verify that the resource quota validation for the namespace fails.")
	secondNamespaceName := namegen.AppendRandomString("testns-")
	secondNamespace, err := namespaces.CreateNamespace(standardUserClient, prq.cluster.ID, createdProject.Name, secondNamespaceName, "", map[string]string{}, map[string]string{})
	require.NoError(prq.T(), err, "Failed to create namespace in the project")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, secondNamespace.Name, namespacePodLimit, false, "Resource quota [pods=4] exceeds project limit")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 0.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, secondNamespace.Name, 0)
	require.NoError(prq.T(), err)

	log.Info("Create a deployment in the first namespace with two replicas and verify that the pods are created.")
	createdFirstDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, firstNamespace.Name, 2, "", "", false, false, false, true)
	require.NoError(prq.T(), err)

	log.Info("Create another deployment in the first namespace with one replica. Verify that the deployment fails to create replicas.")
	createdSecondDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, firstNamespace.Name, 1, "", "", false, false, false, false)
	require.NoError(prq.T(), err)
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkDeploymentStatus(standardUserClient, prq.cluster.ID, firstNamespace.Name, createdSecondDeployment.Name, "ReplicaFailure", "FailedCreate", "forbidden: exceeded quota", 0)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err)

	log.Info("Create a deployment in the second namespace with two replicas. Verify that the deployment fails to create replicas.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, secondNamespace.Name, 2, "", "", false, false, false, false)
	require.NoError(prq.T(), err)
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkDeploymentStatus(standardUserClient, prq.cluster.ID, secondNamespace.Name, createdDeployment.Name, "ReplicaFailure", "FailedCreate", "forbidden: exceeded quota", 0)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err)

	log.Info("Delete the first deployment created in the first namespace.")
	err = deployment.DeleteDeployment(standardUserClient, prq.cluster.ID, createdFirstDeployment)
	require.NoError(prq.T(), err)

	log.Info("Verify that the second deployment created in the first namespace transitions to Active state.")
	updatedDeploymentList, err := deployments.ListDeployments(standardUserClient, prq.cluster.ID, firstNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdSecondDeployment.Name,
	})
	updatedSecondDeployment := updatedDeploymentList.Items[0]
	err = charts.WatchAndWaitDeployments(standardUserClient, prq.cluster.ID, firstNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedSecondDeployment.Name,
	})
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestQuotaPropagationToExistingNamespaces() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quotas) and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the pod limits in the Project spec is accurate.")
	require.Equal(prq.T(), namespacePodLimit, createdProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, createdProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, createdNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Update the resource quota in the Project with new values.")
	namespacePodLimit = "5"
	projectPodLimit = "10"
	projectList, err := projects.ListProjects(standardUserClient, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(prq.T(), err, "Failed to list project.")
	currentProject := projectList.Items[0]
	currentProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	currentProject.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	updatedProject, err := standardUserClient.WranglerContext.Mgmt.Project().Update(&currentProject)
	require.NoError(prq.T(), err, "Failed to update resource quota.")

	log.Info("Verify that the pod limits in the Project spec has the updated values for resource quota.")
	require.Equal(prq.T(), namespacePodLimit, updatedProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, updatedProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace still has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota in the existing namespace has the pod limit in the resource quota still set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Create a new namespace in the project.")
	newNamespaceName := namegen.AppendRandomString("testns-")
	newNamespace, err := namespaces.CreateNamespace(standardUserClient, prq.cluster.ID, updatedProject.Name, newNamespaceName, "", map[string]string{}, map[string]string{})
	require.NoError(prq.T(), err, "Failed to create namespace in the project")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, newNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 5.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, newNamespace.Name, 5)
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestQuotaDeletionPropagationToExistingNamespaces() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quotas) and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the pod limits in the Project spec is accurate.")
	require.Equal(prq.T(), namespacePodLimit, createdProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, createdProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, createdNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Remove the resource quotas set in the Project.")
	namespacePodLimit = ""
	projectPodLimit = ""

	projectList, err := projects.ListProjects(standardUserClient, createdProject.Namespace, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdProject.Name,
	})
	require.NoError(prq.T(), err, "Failed to list project.")
	currentProject := projectList.Items[0]
	currentProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	currentProject.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	updatedProject, err := standardUserClient.WranglerContext.Mgmt.Project().Update(&currentProject)
	require.NoError(prq.T(), err, "Failed to update resource quota.")

	log.Info("Verify that the resource quota in the Project spec has been updated.")
	require.Equal(prq.T(), namespacePodLimit, updatedProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, updatedProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, false)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Verify that the resource quota in the existing namespace is deleted.")
	quotas, err := quotas.ListResourceQuotas(standardUserClient, prq.cluster.ID, createdNamespace.Name, metav1.ListOptions{})
	require.NoError(prq.T(), err)
	require.Empty(prq.T(), quotas)

	log.Info("Create a deployment in the first namespace with ten replicas and verify that the pods are created.")
	_, err = deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 10, "", "", false, false, false, true)
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestOverrideQuotaInNamespace() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quotas) and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the pod limits in the Project spec is accurate.")
	require.Equal(prq.T(), namespacePodLimit, createdProject.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, createdProject.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, createdNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Create a deployment in the namespace with two replicas.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2, "", "", false, false, false, true)
	require.NoError(prq.T(), err)

	log.Info("Override the pod limit for the namespace and increase it from 2 to 3.")
	namespacePodLimit = "3"
	downstreamContext, err := prq.client.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)
	currentNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)
	currentNamespace.Annotations[resourceQuotaAnnotation] = fmt.Sprintf(`{"limit": {"pods": "%s"}}`, namespacePodLimit)
	updatedNamespace, err := downstreamContext.Core.Namespace().Update(currentNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the pod limit for the namespace is set to 3.")
	limitData, err := getNamespaceLimit(standardUserClient, prq.cluster.ID, updatedNamespace.Name, resourceQuotaAnnotation)
	require.NoError(prq.T(), err)
	actualNamespacePodLimit := limitData["limit"].(map[string]interface{})["pods"]
	require.Equal(prq.T(), namespacePodLimit, actualNamespacePodLimit, "Namespace pod limit mismatch")

	log.Info("Verify that the pod limit in the resource quota is set to 3.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, updatedNamespace.Name, 3)
	require.NoError(prq.T(), err)

	log.Info("Increase the number of replicas in the deployment from 2 to 3. Verify that the deployment is in Active state.")
	replicas := int32(3)
	currentDeployment, err := getAndConvertDeployment(standardUserClient, prq.cluster.ID, createdDeployment)
	require.NoError(prq.T(), err)
	currentDeployment.Spec.Replicas = &replicas
	_, err = deployment.UpdateDeployment(standardUserClient, prq.cluster.ID, updatedNamespace.Name, currentDeployment, true)
	require.NoError(prq.T(), err)

	log.Info("Increase the pod limit on the namespace from 3 to 4.")
	namespacePodLimit = "4"
	currentNamespace, err = namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)
	currentNamespace.Annotations[resourceQuotaAnnotation] = fmt.Sprintf(`{"limit": {"pods": "%s"}}`, namespacePodLimit)
	updatedNamespace, err = downstreamContext.Core.Namespace().Update(currentNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota validation for the namespace fails.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, updatedNamespace.Name, namespacePodLimit, false, "Resource quota [pods=4] exceeds project limit")
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestMoveNamespaceFromNoQuotaToQuotaProject() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project in the downstream cluster and a namespace in the project.")
	namespacePodLimit := ""
	projectPodLimit := ""
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Create a deployment in the namespace with ten replicas.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2, "", "", false, false, false, true)
	require.NoError(prq.T(), err)

	log.Info("Create another project in the downstream cluster with resource quota set.")
	namespacePodLimit = "2"
	projectPodLimit = "3"

	projectTemplate := projects.NewProjectTemplate(prq.cluster.ID)
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	projectTemplate.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	createdProject2, err := standardUserClient.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(prq.T(), err, "Failed to create project")

	log.Info("Verify that the project is created and the pod limits in the Project spec is accurate")
	require.Equal(prq.T(), namespacePodLimit, createdProject2.Spec.NamespaceDefaultResourceQuota.Limit.Pods, "Namespace pod limit mismatch")
	require.Equal(prq.T(), projectPodLimit, createdProject2.Spec.ResourceQuota.Limit.Pods, "Project pod limit mismatch")

	log.Info("Move the namespace to the project with resource quota set.")
	downstreamContext, err := prq.client.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)

	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)
	updatedNamespace.Annotations[projects.ProjectIDAnnotation] = createdProject2.Namespace + ":" + createdProject2.Name
	movedNamespace, err := downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (bool, error) {
		err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, movedNamespace.Name, resourceQuotaAnnotation, true)
		if err != nil {
			return false, err
		}

		return true, nil
	})

	log.Info("Verify that the resource quota validation for the namespace is successful.")
	err = checkNamespaceResourceQuotaValidationStatus(standardUserClient, prq.cluster.ID, movedNamespace.Name, namespacePodLimit, true, "")
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, movedNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Verify that increasing the replicas to 3 in the deployment fails with exceeded quota error.")
	replicas := int32(3)
	currentDeployment, err := getAndConvertDeployment(standardUserClient, prq.cluster.ID, createdDeployment)
	require.NoError(prq.T(), err)
	currentDeployment.Spec.Replicas = &replicas
	updatedDeployment, err := deployment.UpdateDeployment(standardUserClient, prq.cluster.ID, updatedNamespace.Name, currentDeployment, false)
	require.NoError(prq.T(), err)

	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkDeploymentStatus(standardUserClient, prq.cluster.ID, movedNamespace.Name, updatedDeployment.Name, "ReplicaFailure", "FailedCreate", "forbidden: exceeded quota", 2)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestMoveNamespaceFromQuotaToNoQuotaProject() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quota) in the downstream cluster and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Create a deployment in the namespace with two replicas.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2, "", "", false, false, false, true)
	require.NoError(prq.T(), err)

	log.Info("Create another project in the downstream cluster without any resource quota set.")
	namespacePodLimit = ""
	projectPodLimit = ""

	projectTemplate := projects.NewProjectTemplate(prq.cluster.ID)
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	projectTemplate.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	createdProject2, err := standardUserClient.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(prq.T(), err, "Failed to create project")

	log.Info("Move the namespace to the project that has no resource quota set.")
	downstreamContext, err := prq.client.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)

	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)
	updatedNamespace.Annotations[projects.ProjectIDAnnotation] = createdProject2.Namespace + ":" + createdProject2.Name
	movedNamespace, err := downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, movedNamespace.Name, resourceQuotaAnnotation, false)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})

	log.Info("Verify that the resource quota object is deleted from the namespace.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, movedNamespace.Name, 0)
	require.Error(prq.T(), err)

	log.Info("Increase the replica count of deployment to 10. Verify that there are 10 pods created in the deployment and they are in Running state.")
	replicas := int32(10)
	currentDeployment, err := getAndConvertDeployment(standardUserClient, prq.cluster.ID, createdDeployment)
	require.NoError(prq.T(), err)
	currentDeployment.Spec.Replicas = &replicas
	_, err = deployment.UpdateDeployment(standardUserClient, prq.cluster.ID, movedNamespace.Name, currentDeployment, true)
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestMoveNamespaceWithDeploymentTransition() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project (with resource quota) in the downstream cluster and a namespace in the project.")
	namespacePodLimit := "2"
	projectPodLimit := "3"
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, true)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should exist")

	log.Info("Verify that the resource quota object is created for the namespace and the pod limit in the resource quota is set to 2.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, createdNamespace.Name, 2)
	require.NoError(prq.T(), err)

	log.Info("Create a deployment in the second namespace with ten replicas.")
	createdDeployment, err := deployment.CreateDeployment(standardUserClient, prq.cluster.ID, createdNamespace.Name, 10, "", "", false, false, false, false)
	require.NoError(prq.T(), err)

	log.Info("Verify that the deployment fails to create ten replicas.")
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkDeploymentStatus(standardUserClient, prq.cluster.ID, createdNamespace.Name, createdDeployment.Name, "ReplicaFailure", "FailedCreate", "forbidden: exceeded quota", 0)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err)

	log.Info("Create another project in the downstream cluster without any resource quota set.")
	namespacePodLimit = ""
	projectPodLimit = ""

	projectTemplate := projects.NewProjectTemplate(prq.cluster.ID)
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	projectTemplate.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	createdProject2, err := standardUserClient.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(prq.T(), err, "Failed to create project")

	log.Info("Move the namespace to the project that has no resource quota set.")
	downstreamContext, err := prq.client.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)

	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)
	updatedNamespace.Annotations[projects.ProjectIDAnnotation] = createdProject2.Namespace + ":" + createdProject2.Name
	movedNamespace, err := downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		checkErr := checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, movedNamespace.Name, resourceQuotaAnnotation, false)
		if checkErr != nil {
			return false, checkErr
		}

		return true, nil
	})
	require.NoError(prq.T(), err)

	log.Info("Verify that the resource quota object is deleted from the namespace.")
	err = checkNamespaceResourceQuota(standardUserClient, prq.cluster.ID, updatedNamespace.Name, 0)
	require.Error(prq.T(), err)

	log.Info("Verify that there are 10 pods created in the deployment and they are in Running state.")
	err = charts.WatchAndWaitDeployments(standardUserClient, prq.cluster.ID, updatedNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDeployment.Name,
	})
	require.NoError(prq.T(), err)
}

func (prq *ProjectsResourceQuotaTestSuite) TestMoveNamespaceBetweenProjectsWithNoResourceQuota() {
	subSession := prq.session.NewSession()
	defer subSession.Cleanup()

	standardUserClient, _ := prq.setupUserForProject()

	log.Info("Create a project in the downstream cluster and a namespace in the project.")
	namespacePodLimit := ""
	projectPodLimit := ""
	createdProject, createdNamespace, err := createProjectAndNamespaceWithQuotas(standardUserClient, prq.cluster.ID, namespacePodLimit, projectPodLimit)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the label and annotation referencing the project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject.Name, createdNamespace.Name)
	require.NoError(prq.T(), err)
	updatedNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, createdNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, createdNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Create a deployment in the namespace with ten replicas.")
	deployment, err := deployment.CreateDeployment(standardUserClient, createdProject.Namespace, createdNamespace.Name, 10, "", "", false, false, false, true)
	require.NoError(prq.T(), err)

	log.Info("Create another project in the downstream cluster.")
	projectTemplate := projects.NewProjectTemplate(prq.cluster.ID)
	createdProject2, err := standardUserClient.WranglerContext.Mgmt.Project().Create(projectTemplate)
	require.NoError(prq.T(), err, "Failed to create project")
	err = project.WaitForProjectFinalizerToUpdate(prq.client, createdProject2.Name, createdProject2.Namespace, 2)
	require.NoError(prq.T(), err)

	log.Info("Move the namespace from the first project to the second project.")
	currentNamespace, err := namespaces.GetNamespaceByName(standardUserClient, prq.cluster.ID, updatedNamespace.Name)
	require.NoError(prq.T(), err)
	downstreamContext, err := prq.client.WranglerContext.DownStreamClusterWranglerContext(prq.cluster.ID)
	require.NoError(prq.T(), err)

	updatedNamespace.Annotations[projects.ProjectIDAnnotation] = createdProject2.Namespace + ":" + createdProject2.Name
	updatedNamespace.ResourceVersion = currentNamespace.ResourceVersion
	_, err = downstreamContext.Core.Namespace().Update(updatedNamespace)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace has the correct label and annotation referencing the second project.")
	err = project.WaitForProjectIDUpdate(standardUserClient, prq.cluster.ID, createdProject2.Name, updatedNamespace.Name)
	require.NoError(prq.T(), err)

	log.Info("Verify that the namespace does not have the annotation: field.cattle.io/resourceQuota.")
	err = checkAnnotationExistsInNamespace(standardUserClient, prq.cluster.ID, updatedNamespace.Name, resourceQuotaAnnotation, false)
	require.NoError(prq.T(), err, "'field.cattle.io/resourceQuota' annotation should not exist")

	log.Info("Verify that the deployment is in Active state and all pods in the deployment are in Running state.")
	err = charts.WatchAndWaitDeployments(standardUserClient, prq.cluster.ID, updatedNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	require.NoError(prq.T(), err)
}

func TestProjectsResourceQuotaTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectsResourceQuotaTestSuite))
}
