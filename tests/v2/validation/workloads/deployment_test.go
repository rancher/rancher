//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
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

type DeploymentTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (d *DeploymentTestSuite) TearDownSuite() {
	d.session.Cleanup()
}

func (d *DeploymentTestSuite) SetupSuite() {
	d.session = session.NewSession()

	client, err := rancher.NewClient("", d.session)
	require.NoError(d.T(), err)

	d.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(d.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(d.client, clusterName)
	require.NoError(d.T(), err, "Error getting cluster ID")

	d.cluster, err = d.client.Management.Cluster.ByID(clusterID)
	require.NoError(d.T(), err)
}

func (d *DeploymentTestSuite) TestDeploymentSideKick() {
	subSession := d.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(d.client, d.cluster.ID)
	require.NoError(d.T(), err)

	log.Info("Creating new deployment")
	createdDeployment, err := deployment.CreateDeployment(d.client, d.cluster.ID, namespace.Name, 1, "", "", false, false, true)
	require.NoError(d.T(), err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(d.client, d.cluster.ID, namespace.Name, createdDeployment)
	require.NoError(d.T(), err)

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
	updatedDeployment, err := deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, createdDeployment, true)
	require.NoError(d.T(), err)

	log.Info("Waiting deployment comes up active")
	err = charts.WatchAndWaitDeployments(d.client, d.cluster.ID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	require.NoError(d.T(), err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(d.client, d.cluster.ID, namespace.Name, updatedDeployment)
	require.NoError(d.T(), err)
}

func (d *DeploymentTestSuite) TestDeploymentUpgrade() {
	subSession := d.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(d.client, d.cluster.ID)
	require.NoError(d.T(), err)

	log.Info("Creating new deployment")
	upgradeDeployment, err := deployment.CreateDeployment(d.client, d.cluster.ID, namespace.Name, 2, "", "", false, false, true)
	require.NoError(d.T(), err)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "1", nginxImageName, 2)

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
	upgradeDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, upgradeDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "2", redisImageName, 2)

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
	_, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, upgradeDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "3", ubuntuImageName, 2)

	log.Info("Rollback deployment")
	logRollback, err := rollbackDeployment(d.client, d.cluster.ID, namespace.Name, upgradeDeployment.Name, 1)
	require.NoError(d.T(), err)
	require.NotEmpty(d.T(), logRollback)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "4", nginxImageName, 2)

	log.Info("Rollback deployment")
	logRollback, err = rollbackDeployment(d.client, d.cluster.ID, namespace.Name, upgradeDeployment.Name, 2)
	require.NoError(d.T(), err)
	require.NotEmpty(d.T(), logRollback)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "5", redisImageName, 2)

	log.Info("Rollback deployment")
	logRollback, err = rollbackDeployment(d.client, d.cluster.ID, namespace.Name, upgradeDeployment.Name, 3)
	require.NoError(d.T(), err)
	require.NotEmpty(d.T(), logRollback)

	validateDeploymentUpgrade(d.T(), d.client, d.cluster.ID, namespace.Name, upgradeDeployment, "6", ubuntuImageName, 2)
}

func (d *DeploymentTestSuite) TestDeploymentPodScaleUp() {
	subSession := d.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(d.client, d.cluster.ID)
	require.NoError(d.T(), err)

	log.Info("Creating new deployment")
	scaleUpDeployment, err := deployment.CreateDeployment(d.client, d.cluster.ID, namespace.Name, 1, "", "", false, false, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 1)

	replicas := int32(2)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, scaleUpDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 2)

	replicas = int32(3)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, scaleUpDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleUpDeployment, nginxImageName, 3)
}

func (d *DeploymentTestSuite) TestDeploymentPodScaleDown() {
	subSession := d.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(d.client, d.cluster.ID)
	require.NoError(d.T(), err)

	log.Info("Creating new deployment")
	scaleDownDeployment, err := deployment.CreateDeployment(d.client, d.cluster.ID, namespace.Name, 3, "", "", false, false, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 3)

	replicas := int32(2)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, scaleDownDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 2)

	replicas = int32(1)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, scaleDownDeployment, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, scaleDownDeployment, nginxImageName, 1)
}

func (d *DeploymentTestSuite) TestDeploymentPauseOrchestration() {
	subSession := d.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(d.client, d.cluster.ID)
	require.NoError(d.T(), err)

	log.Info("Creating new deployment")
	pauseDeployment, err := deployment.CreateDeployment(d.client, d.cluster.ID, namespace.Name, 2, "", "", false, false, true)
	require.NoError(d.T(), err)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, pauseDeployment, nginxImageName, 2)

	log.Info("Pausing orchestration")
	pauseDeployment.Spec.Paused = true
	pauseDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, pauseDeployment, true)
	require.NoError(d.T(), err)

	log.Info("Verifying orchestration is paused")
	err = verifyOrchestrationStatus(d.client, d.cluster.ID, namespace.Name, pauseDeployment.Name, true)
	require.NoError(d.T(), err)

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
	pauseDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, pauseDeployment, true)
	require.NoError(d.T(), err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(d.client, d.cluster.ID, namespace.Name, pauseDeployment)
	require.NoError(d.T(), err)

	log.Info("Verifying that the deployment was not updated and the replica count was increased")
	log.Infof("Counting all pods running by image %s", nginxImageName)
	countPods, err := pods.CountPodContainerRunningByImage(d.client, d.cluster.ID, namespace.Name, nginxImageName)
	require.NoError(d.T(), err)
	require.Equal(d.T(), int(replicas), countPods)

	log.Info("Activing orchestration")
	pauseDeployment.Spec.Paused = false
	pauseDeployment, err = deployment.UpdateDeployment(d.client, d.cluster.ID, namespace.Name, pauseDeployment, true)

	validateDeploymentScale(d.T(), d.client, d.cluster.ID, namespace.Name, pauseDeployment, redisImageName, int(replicas))

	log.Info("Verifying orchestration is active")
	err = verifyOrchestrationStatus(d.client, d.cluster.ID, namespace.Name, pauseDeployment.Name, false)
	require.NoError(d.T(), err)

	log.Infof("Counting all pods running by image %s", redisImageName)
	countPods, err = pods.CountPodContainerRunningByImage(d.client, d.cluster.ID, namespace.Name, redisImageName)
	require.NoError(d.T(), err)
	require.Equal(d.T(), int(replicas), countPods)
}

func TestDeploymentTestSuite(t *testing.T) {
	suite.Run(t, new(DeploymentTestSuite))
}
