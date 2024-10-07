//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nginxImageName  = "nginx"
	ubuntuImageName = "ubuntu"
	redisImageName  = "redis"
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

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	_, err = deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadSideKick() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	createdDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(w.client, w.cluster.ID, namespace.Name, createdDeployment)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	createdDeployment.Spec.Template.Spec.Containers = append(createdDeployment.Spec.Template.Spec.Containers, newContainerTemplate)

	log.Info("Updating image deployment")
	updatedDeployment, err := deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, createdDeployment)
	require.NoError(w.T(), err)

	log.Info("Waiting deployment comes up active")
	err = charts.WatchAndWaitDeployments(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	require.NoError(w.T(), err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(w.client, w.cluster.ID, namespace.Name, updatedDeployment)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDaemonSet() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deamonset")
	createdDaemonset, err := daemonset.CreateDaemonset(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)

	log.Info("Waiting deamonset comes up active")
	err = charts.WatchAndWaitDaemonSets(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonset.Name,
	})
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadCronjob() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImageName,
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

	log.Info("Creating new cronjob")
	cronJobTemplate, err := cronjob.CreateCronjob(w.client, w.cluster.ID, namespace.Name, "*/1 * * * *", podTemplate)
	require.NoError(w.T(), err)

	log.Info("Waiting cronjob comes up active")
	err = cronjob.WatchAndWaitCronjob(w.client, w.cluster.ID, namespace.Name, cronJobTemplate)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadStatefulset() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImageName,
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

	log.Info("Creating new statefulset")
	statefulsetTemplate, err := statefulset.CreateStatefulset(w.client, w.cluster.ID, namespace.Name, podTemplate, 1)
	require.NoError(w.T(), err)

	log.Info("Waiting statefulset comes up active")
	err = charts.WatchAndWaitStatefulSets(w.client, w.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + statefulsetTemplate.Name,
	})
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadUpgrade() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	upgradeDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 2, "", "", false, false)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "1", nginxImageName, 2)

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment")
	upgradeDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "2", redisImageName, 2)

	containerName = namegen.AppendRandomString("update-test-container-two")
	newContainerTemplate = workloads.NewContainer(containerName,
		ubuntuImageName,
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

	log.Info("Updating deployment")
	_, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment)
	require.NoError(w.T(), err)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "3", ubuntuImageName, 2)

	log.Info("Rollbacking deployment")
	logRollback, err := rollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 1)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "4", nginxImageName, 2)

	log.Info("Rollbacking deployment")
	logRollback, err = rollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 2)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "5", redisImageName, 2)

	log.Info("Rollbacking deployment")
	logRollback, err = rollbackDeployment(w.client, w.cluster.ID, namespace.Name, upgradeDeployment.Name, 3)
	require.NoError(w.T(), err)
	require.NotEmpty(w.T(), logRollback)

	validateDeploymentUpgrade(w.T(), w.client, w.cluster.ID, namespace.Name, upgradeDeployment, "6", ubuntuImageName, 2)
}

func (w *WorkloadTestSuite) TestWorkloadPodScaleUp() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	scaleUpDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 1)

	replicas := int32(2)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, scaleUpDeployment)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 2)

	replicas = int32(3)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, scaleUpDeployment)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 3)
}

func (w *WorkloadTestSuite) TestWorkloadPodScaleDown() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	scaleDownDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 3, "", "", false, false)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 3)

	replicas := int32(2)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, scaleDownDeployment)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 2)

	replicas = int32(1)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, scaleDownDeployment)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 1)
}

func (w *WorkloadTestSuite) TestWorkloadPauseOrchestration() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	log.Info("Creating new deployment")
	pauseDeployment, err := deployment.CreateDeploymentWithConfigmap(w.client, w.cluster.ID, namespace.Name, 2, "", "", false, false)
	require.NoError(w.T(), err)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, pauseDeployment, nginxImageName, 2)

	log.Info("Pausing orchestration")
	pauseDeployment.Spec.Paused = true
	pauseDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, pauseDeployment)
	require.NoError(w.T(), err)

	log.Info("Verifying orchestration is paused")
	err = verifyOrchestrationStatus(w.client, w.cluster.ID, namespace.Name, pauseDeployment.Name, true)
	require.NoError(w.T(), err)

	replicas := int32(3)
	pauseDeployment.Spec.Replicas = &replicas
	containerName := namegen.AppendRandomString("pause-redis-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	pauseDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment image and replica")
	pauseDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, pauseDeployment)
	require.NoError(w.T(), err)

	log.Info("Verifying that the deployment was not updated and the replica count was increased")
	log.Infof("Counting all pods running by image %s", nginxImageName)
	countPods, err := pods.CountPodContainerRunningByImage(w.client, w.cluster.ID, namespace.Name, nginxImageName)
	require.NoError(w.T(), err)
	require.Equal(w.T(), int(replicas), countPods)

	log.Info("Activing orchestration")
	pauseDeployment.Spec.Paused = false
	pauseDeployment, err = deployment.UpdateDeployment(w.client, w.cluster.ID, namespace.Name, pauseDeployment)

	validateDeploymentScale(w.T(), w.client, w.cluster.ID, namespace.Name, pauseDeployment, redisImageName, int(replicas))

	log.Info("Verifying orchestration is active")
	err = verifyOrchestrationStatus(w.client, w.cluster.ID, namespace.Name, pauseDeployment.Name, false)
	require.NoError(w.T(), err)

	log.Infof("Counting all pods running by image %s", redisImageName)
	countPods, err = pods.CountPodContainerRunningByImage(w.client, w.cluster.ID, namespace.Name, redisImageName)
	require.NoError(w.T(), err)
	require.Equal(w.T(), int(replicas), countPods)
}

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
