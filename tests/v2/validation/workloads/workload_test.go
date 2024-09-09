//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	pods "github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/rancher/tests/v2/actions/workloads/statefulset"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkloadTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (w *WorkloadTestSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *WorkloadTestSuite) SetupSuite() {
	w.session = session.NewSession()

	client, err := rancher.NewClient("", w.session)
	require.NoError(w.T(), err)

	w.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(w.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(w.client, clusterName)
	require.NoError(w.T(), err, "Error getting cluster ID")

	w.cluster, err = w.client.Management.Cluster.ByID(clusterID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDeployment() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	_, err = deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadSideKick() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	createdDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)

	err = pods.WatchAndWaitPodContainerRunning(w.client, w.cluster.ID, namespace.Name, createdDeployment)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("updatetestcontainer")
	newContainerTemplate := workloads.NewContainer(containerName,
		"redis",
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	createdDeployment.Spec.Template.Spec.Containers = append(createdDeployment.Spec.Template.Spec.Containers, newContainerTemplate)

	updatedDeployment, err := deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, createdDeployment)
	require.NoError(w.T(), err)

	err = charts.WatchAndWaitDeployments(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	require.NoError(w.T(), err)

	err = pods.WatchAndWaitPodContainerRunning(w.client, w.cluster.ID, namespace.Name, updatedDeployment)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDaemonSet() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	createdDaemonset, err := daemonset.CreateDaemonset(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)

	err = charts.WatchAndWaitDaemonSets(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonset.Name,
	})
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadCronjob() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		"nginx",
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	cronJobTemplate, err := cronjob.CreateCronjob(w.client, w.cluster.ID, namespace.Name, "*/1 * * * *", podTemplate)
	require.NoError(w.T(), err)

	err = cronjob.WatchAndWaitCronjob(w.client, w.cluster.ID, namespace.Name, cronJobTemplate)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadStatefulset() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		"nginx",
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	statefulsetTemplate, err := statefulset.CreateStatefulset(w.client, w.cluster.ID, namespace.Name, podTemplate, 1)
	require.NoError(w.T(), err)

	err = charts.WatchAndWaitStatefulSets(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + statefulsetTemplate.Name,
	})
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadUpgrade() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	upgradeDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 2, "", "", false, false)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "1", "nginx", 2)

	containerName := namegen.AppendRandomString("updatetestcontainer")
	newContainerTemplate := workloads.NewContainer(containerName,
		"redis",
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	upgradeDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "2", "redis", 2)

	containerName = namegen.AppendRandomString("updatetestcontainertwo")
	newContainerTemplate = workloads.NewContainer(containerName,
		"ubuntu",
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	newContainerTemplate.TTY = true
	newContainerTemplate.Stdin = true
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	_, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "3", "ubuntu", 2)

	logRollback, err := deployment.RollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 1)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "4", "nginx", 2)

	logRollback, err = deployment.RollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 2)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "5", "redis", 2)

	logRollback, err = deployment.RollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 3)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w, namespace.Name, upgradeDeployment, "6", "ubuntu", 2)
}

func validateDeploymentUpgrade(w *WorkloadTestSuite, namespaceName string, appv1Deployment *appv1.Deployment, expectedRevision string, image string, expectedContainerCount int) {
	err := charts.WatchAndWaitDeployments(w.client, w.cluster.ID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + appv1Deployment.Name,
	})
	require.NoError(w.T(), err)

	err = pods.WatchAndWaitPodContainerRunning(w.client, w.cluster.ID, namespaceName, appv1Deployment)
	require.NoError(w.T(), err)

	err = deployment.ValidateRolloutHistoryDeployment(w.client, w.cluster.ID, namespaceName, appv1Deployment.Name, expectedRevision)
	require.NoError(w.T(), err)

	countPods, err := pods.CountPodContainerRunningByImage(w.client, w.cluster.ID, namespaceName, image)
	require.NoError(w.T(), err)
	require.Equal(w.T(), expectedContainerCount, countPods)
}

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
