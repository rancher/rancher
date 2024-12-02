//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package clusterandprojectroles

import (
	"fmt"
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManageWorkloadsRoleTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (mw *ManageWorkloadsRoleTestSuite) TearDownSuite() {
	mw.session.Cleanup()
}

func (mw *ManageWorkloadsRoleTestSuite) SetupSuite() {
	mw.session = session.NewSession()

	client, err := rancher.NewClient("", mw.session)
	require.NoError(mw.T(), err)
	mw.client = client

	log.Info("Getting cluster name from the config file and append cluster details in mw")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(mw.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(mw.client, clusterName)
	require.NoError(mw.T(), err, "Error getting cluster ID")
	mw.cluster, err = mw.client.Management.Cluster.ByID(clusterID)
	require.NoError(mw.T(), err)
}

func (mw *ManageWorkloadsRoleTestSuite) testSetupUserAndProject() (*rancher.Client, *management.Project, *corev1.Namespace) {
	log.Info("Create a standard user.")
	newUser, standardUserClient, err := rbac.SetupUser(mw.client, rbac.StandardUser.String())
	require.NoError(mw.T(), err)

	log.Info("Create a project and a namespace")
	createdProject, createdNamespace, err := projectsapi.CreateProjectAndNamespace(mw.client, mw.cluster.ID)
	require.NoError(mw.T(), err)

	log.Infof("Add the user %s as Project Owner to the project %s", newUser.Name, createdProject.Name)
	errUserRole := users.AddProjectMember(mw.client, createdProject, newUser, rbac.ProjectOwner.String(), nil)
	require.NoError(mw.T(), errUserRole)

	standardUserClient, err = standardUserClient.ReLogin()
	require.NoError(mw.T(), err)

	return standardUserClient, createdProject, createdNamespace
}

func (mw *ManageWorkloadsRoleTestSuite) testSetupWorkloadUserAndAddToProject(adminProject *management.Project) (*management.User, *rancher.Client) {
	log.Info("Create a new standard user.")
	workloadUser, workloadUserClient, err := rbac.SetupUser(mw.client, rbac.StandardUser.String())
	require.NoError(mw.T(), err, "Failed to create a new standard user.")

	log.Infof("Verify that the project owner is able to add the new user %s to the project %s with 'Manage Workloads' role.", workloadUser.Username, adminProject.Name)
	errUserRole := users.AddProjectMember(mw.client, adminProject, workloadUser, rbac.ManageWorkloads.String(), nil)
	require.NoError(mw.T(), errUserRole, "Project owner failed to add the new user to the project with 'Manage Workloads' role.")

	log.Infof("Login as user %s.", workloadUser.Name)
	workloadUserClient, err = workloadUserClient.ReLogin()
	require.NoError(mw.T(), err, "Failed to log in as the new workload user.")

	return workloadUser, workloadUserClient
}

func (mw *ManageWorkloadsRoleTestSuite) TestManageWorkloadsPermissions() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Verify that the 'Manage Workloads' role has the correct verbs for the resources.")
	expectedVerbs := []string{
		"get",
		"list",
		"watch",
		"create",
		"delete",
		"deletecollection",
		"patch",
		"update",
	}

	expectedRules := map[string][]string{
		"pods":                         expectedVerbs,
		"pods/attach":                  expectedVerbs,
		"pods/exec":                    expectedVerbs,
		"pods/portforward":             expectedVerbs,
		"pods/proxy":                   expectedVerbs,
		"replicationcontrollers":       expectedVerbs,
		"replicationcontrollers/scale": expectedVerbs,
		"daemonsets":                   expectedVerbs,
		"deployments":                  expectedVerbs,
		"deployments/scale":            expectedVerbs,
		"replicasets":                  expectedVerbs,
		"replicasets/scale":            expectedVerbs,
		"statefulsets":                 expectedVerbs,
		"statefulsets/scale":           expectedVerbs,
		"deployments/rollback":         {"create", "delete", "deletecollection", "patch", "update"},
		"horizontalpodautoscalers":     expectedVerbs,
		"cronjobs":                     expectedVerbs,
		"jobs":                         expectedVerbs,
	}

	roleTemplate, err := mw.client.Management.RoleTemplate.ByID(rbac.ManageWorkloads.String())
	require.NoError(mw.T(), err, "failed to fetch role template")

	actualRules := make(map[string][]string)
	for _, rule := range roleTemplate.Rules {
		for _, resource := range rule.Resources {
			actualRules[resource] = rule.Verbs
		}
	}

	err = rbac.VerifyRoleRules(expectedRules, actualRules)
	require.NoError(mw.T(), err, "role rules verification failed")
}

func (mw *ManageWorkloadsRoleTestSuite) TestProjectOwnerAssignsManageWorkloadsRole() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	_, adminProject, _ := mw.testSetupUserAndProject()

	_, _ = mw.testSetupWorkloadUserAndAddToProject(adminProject)
}

func (mw *ManageWorkloadsRoleTestSuite) TestManageWorkloadsRoleForPods() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	_, adminProject, adminNamespace := mw.testSetupUserAndProject()

	workloadUser, workloadUserClient := mw.testSetupWorkloadUserAndAddToProject(adminProject)

	log.Infof("As user %s, Create a new pod in the namespace within the project %s.", workloadUser.Name, adminProject.Name)
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		rbac.ImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil, nil, nil,
	)

	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil, nil,
	)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namegen.AppendRandomString("testpod"),
			Namespace: adminNamespace.Name,
			Labels:    podTemplate.Labels,
		},
		Spec: podTemplate.Spec,
	}

	downstreamContext, err := workloadUserClient.WranglerContext.DownStreamClusterWranglerContext(mw.cluster.ID)
	require.NoError(mw.T(), err)
	createdPod, err := downstreamContext.Core.Pod().Create(pod)
	require.NoError(mw.T(), err, "Failed to create the pod.")
	err = pods.WatchAndWaitPodContainerRunning(workloadUserClient, mw.cluster.ID, adminNamespace.Name, nil)
	require.NoError(mw.T(), err)

	log.Infof("As user %s, list the pod %s.", workloadUser.Username, createdPod.Name)
	listPod, err := downstreamContext.Core.Pod().List(adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdPod.Name,
	})
	require.NoError(mw.T(), err, "Failed to list pods.")
	require.Len(mw.T(), listPod.Items, 1, "Expected exactly one pod to be listed.")

	log.Infof("As user %s, get the pod %s.", workloadUser.Username, createdPod.Name)
	getPod, err := downstreamContext.Core.Pod().Get(adminNamespace.Name, createdPod.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the pod.")
	log.Infof("Pod %s has status %s.", getPod.Name, getPod.Status.Phase)

	log.Infof("As user %s, update the pod %s with a new label.", workloadUser.Username, createdPod.Name)
	if getPod.Labels == nil {
		getPod.Labels = make(map[string]string)
	}
	getPod.Labels["updated"] = "true"
	updatedPod, err := downstreamContext.Core.Pod().Update(getPod)
	require.NoError(mw.T(), err, "Failed to update the pod.")
	require.Equal(mw.T(), "true", updatedPod.Labels["updated"], "Pod label update failed.")

	log.Infof("As user %s, delete the pod %s.", workloadUser.Username, createdPod.Name)
	err = downstreamContext.Core.Pod().Delete(adminNamespace.Name, updatedPod.Name, nil)
	require.NoError(mw.T(), err, "Failed to delete the pod.")
}

func (mw *ManageWorkloadsRoleTestSuite) TestManageWorkloadsRoleForDeployments() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	_, adminProject, adminNamespace := mw.testSetupUserAndProject()

	workloadUser, workloadUserClient := mw.testSetupWorkloadUserAndAddToProject(adminProject)

	log.Infof("As user %s, create a new deployment in the namespace within the project %s.", workloadUser.Name, adminProject.Name)
	createdDeployment, err := deployment.CreateDeployment(workloadUserClient, mw.cluster.ID, adminNamespace.Name, 1, "", "", false, false, false, true)
	require.NoError(mw.T(), err, "Failed to create the deployment.")

	log.Infof("As user %s, list the deployment %s.", workloadUser.Username, createdDeployment.Name)
	downstreamContext, err := workloadUserClient.WranglerContext.DownStreamClusterWranglerContext(mw.cluster.ID)
	require.NoError(mw.T(), err)
	listDeployment, err := downstreamContext.Apps.Deployment().List(adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDeployment.Name,
	})
	require.NoError(mw.T(), err, "Failed to list deployments.")
	require.Len(mw.T(), listDeployment.Items, 1, "Expected exactly one deployment to be listed.")

	log.Infof("As user %s, get the deployment %s.", workloadUser.Username, createdDeployment.Name)
	getDeployment, err := downstreamContext.Apps.Deployment().Get(adminNamespace.Name, createdDeployment.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the deployment.")

	log.Infof("As user %s, update the deployment %s and scale up the replicas.", workloadUser.Username, createdDeployment.Name)
	replicas := int32(2)
	getDeployment.Spec.Replicas = &replicas
	updatedDeployment, err := deployment.UpdateDeployment(workloadUserClient, mw.cluster.ID, adminNamespace.Name, getDeployment, true)
	require.NoError(mw.T(), err)

	updatedDeployment, err = downstreamContext.Apps.Deployment().Get(adminNamespace.Name, updatedDeployment.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the updated deployment after scaling.")
	require.Equal(mw.T(), replicas, *updatedDeployment.Spec.Replicas, "Replica count did not match after scaling up.")

	log.Infof("As user %s, delete the deployment %s.", workloadUser.Username, createdDeployment.Name)
	err = deployment.DeleteDeployment(workloadUserClient, mw.cluster.ID, updatedDeployment)
	require.NoError(mw.T(), err, "Failed to delete the deployment.")
}

func (mw *ManageWorkloadsRoleTestSuite) TestManageWorkloadsRoleForDaemonSets() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	_, adminProject, adminNamespace := mw.testSetupUserAndProject()

	workloadUser, workloadUserClient := mw.testSetupWorkloadUserAndAddToProject(adminProject)

	log.Infof("As user %s, create a new DaemonSet in the namespace within the project %s.", workloadUser.Name, adminProject.Name)
	createdDaemonSet, err := daemonset.CreateDaemonset(workloadUserClient, mw.cluster.ID, adminNamespace.Name, 1, "", "", false, false)
	require.NoError(mw.T(), err, "Failed to create the DaemonSet.")
	err = charts.WatchAndWaitDaemonSets(workloadUserClient, mw.cluster.ID, adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonSet.Name,
	})
	require.NoError(mw.T(), err)

	log.Infof("As user %s, list the DaemonSet %s.", workloadUser.Username, createdDaemonSet.Name)
	downstreamContext, err := workloadUserClient.WranglerContext.DownStreamClusterWranglerContext(mw.cluster.ID)
	require.NoError(mw.T(), err)
	listDaemonSet, err := downstreamContext.Apps.DaemonSet().List(adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonSet.Name,
	})
	require.NoError(mw.T(), err, "Failed to list DaemonSets.")
	require.Len(mw.T(), listDaemonSet.Items, 1, "Expected exactly one DaemonSet to be listed.")

	log.Infof("As user %s, get the DaemonSet %s.", workloadUser.Username, createdDaemonSet.Name)
	getDaemonSet, err := downstreamContext.Apps.DaemonSet().Get(adminNamespace.Name, createdDaemonSet.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the DaemonSet.")

	log.Infof("As user %s, update the DaemonSet %s with a new label.", workloadUser.Username, createdDaemonSet.Name)
	if getDaemonSet.Labels == nil {
		getDaemonSet.Labels = make(map[string]string)
	}
	getDaemonSet.Labels["updated"] = "true"
	updatedDaemonSet, err := daemonset.UpdateDaemonset(workloadUserClient, mw.cluster.ID, adminNamespace.Name, getDaemonSet)
	require.NoError(mw.T(), err)
	err = charts.WatchAndWaitDaemonSets(workloadUserClient, mw.cluster.ID, adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDaemonSet.Name,
	})
	require.NoError(mw.T(), err)

	updatedDaemonSet, err = downstreamContext.Apps.DaemonSet().Get(adminNamespace.Name, updatedDaemonSet.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the updated DaemonSet after updating labels.")
	require.Equal(mw.T(), "true", updatedDaemonSet.Labels["updated"], "DaemonSet label update failed.")

	log.Infof("As user %s, delete the DaemonSet %s.", workloadUser.Username, updatedDaemonSet.Name)
	err = daemonset.DeleteDaemonset(workloadUserClient, mw.cluster.ID, updatedDaemonSet)
	require.NoError(mw.T(), err, "Failed to delete the DaemonSet.")
}

func (mw *ManageWorkloadsRoleTestSuite) TestManageWorkloadsRoleForStatefulSets() {
	subSession := mw.session.NewSession()
	defer subSession.Cleanup()

	_, adminProject, adminNamespace := mw.testSetupUserAndProject()

	workloadUser, workloadUserClient := mw.testSetupWorkloadUserAndAddToProject(adminProject)

	log.Infof("As user %s, create a new StatefulSet in the namespace within the project %s.", workloadUser.Name, adminProject.Name)
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways
	replicas := int32(1)
	containerTemplate := workloads.NewContainer(
		containerName,
		rbac.ImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	statefulsetName := namegen.AppendRandomString("teststatefulset")
	labels := map[string]string{
		"workload.user.cattle.io/workloadselector": fmt.Sprintf("apps.statefulset-%v-%v", adminNamespace.Name, statefulsetName),
	}
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)
	podTemplate.ObjectMeta = metav1.ObjectMeta{
		Labels: labels,
	}

	statefulset := &appsV1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulsetName,
			Namespace: adminNamespace.Name,
			Labels: map[string]string{
				"workload.user.cattle.io/workloadselector": fmt.Sprintf("apps.statefulset-%v-%v", adminNamespace.Name, statefulsetName),
			},
		},
		Spec: appsV1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"workload.user.cattle.io/workloadselector": fmt.Sprintf("apps.statefulset-%v-%v", adminNamespace.Name, statefulsetName),
				},
			},
			Template: podTemplate,
		},
	}

	downstreamContext, err := workloadUserClient.WranglerContext.DownStreamClusterWranglerContext(mw.cluster.ID)
	require.NoError(mw.T(), err)
	createdStatefulset, err := downstreamContext.Apps.StatefulSet().Create(statefulset)
	require.NoError(mw.T(), err, "Failed to create the StatefulSet.")
	err = charts.WatchAndWaitStatefulSets(workloadUserClient, mw.cluster.ID, adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdStatefulset.Name,
	})
	require.NoError(mw.T(), err)

	log.Infof("As user %s, list the StatefulSet %s.", workloadUser.Username, createdStatefulset.Name)
	listStatefulset, err := downstreamContext.Apps.StatefulSet().List(adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdStatefulset.Name,
	})
	require.NoError(mw.T(), err, "Failed to list StatefulSets.")
	require.Len(mw.T(), listStatefulset.Items, 1, "Expected exactly one StatefulSet to be listed.")

	log.Infof("As user %s, get the StatefulSet %s.", workloadUser.Username, createdStatefulset.Name)
	getStatefulset, err := downstreamContext.Apps.StatefulSet().Get(adminNamespace.Name, createdStatefulset.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the StatefulSet.")

	log.Infof("As user %s, update the StatefulSet %s with a new label.", workloadUser.Username, createdStatefulset.Name)
	if getStatefulset.Labels == nil {
		getStatefulset.Labels = make(map[string]string)
	}
	getStatefulset.Labels["updated"] = "true"
	updatedStatefulset, err := downstreamContext.Apps.StatefulSet().Update(getStatefulset)
	require.NoError(mw.T(), err, "Failed to update the StatefulSet.")
	updatedStatefulset, err = downstreamContext.Apps.StatefulSet().Get(adminNamespace.Name, updatedStatefulset.Name, metav1.GetOptions{})
	require.NoError(mw.T(), err, "Failed to get the updated StatefulSet after updating labels.")
	require.Equal(mw.T(), "true", updatedStatefulset.Labels["updated"], "StatefulSet label update failed.")

	err = charts.WatchAndWaitStatefulSets(workloadUserClient, mw.cluster.ID, adminNamespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdStatefulset.Name,
	})
	require.NoError(mw.T(), err)

	log.Infof("As user %s, delete the StatefulSet %s.", workloadUser.Username, createdStatefulset.Name)
	err = downstreamContext.Apps.StatefulSet().Delete(adminNamespace.Name, createdStatefulset.Name, nil)
	require.NoError(mw.T(), err, "Failed to delete the StatefulSet.")
}

func TestManageWorkloadsRoleTestSuite(t *testing.T) {
	suite.Run(t, new(ManageWorkloadsRoleTestSuite))
}
