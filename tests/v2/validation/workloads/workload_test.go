//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
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
	_, err = deployment.CreateDeployment(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
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

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
