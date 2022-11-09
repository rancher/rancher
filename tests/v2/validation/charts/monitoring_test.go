package charts

import (
	"fmt"
	"math/rand"
	"testing"

	"net/url"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/services"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
)

type MonitoringTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	project             *management.Project
	chartInstallOptions *charts.InstallOptions
	chartFeatureOptions *charts.RancherMonitoringOpts
}

func (m *MonitoringTestSuite) TearDownSuite() {
	m.session.Cleanup()
}

func (m *MonitoringTestSuite) SetupSuite() {
	testSession := session.NewSession(m.T())
	m.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(m.T(), err)

	m.client = client

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(m.T(), clusterName, "Cluster name to install is not set")

	// Get clusterID with clusterName
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(m.T(), err)

	// Change alert manager and grafana paths if it's not local cluster
	if clusterID != clusterName {
		alertManagerPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, alertManagerPath)
		grafanaPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, grafanaPath)
		prometheusTargetsPathAPI = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, prometheusTargetsPathAPI)
	}

	// Change prometheus paths to use the clusterID
	prometheusGraphPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, prometheusGraphPath)
	prometheusRulesPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, prometheusRulesPath)
	prometheusTargetsPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, prometheusTargetsPath)

	// Get latest versions of the monitoring chart
	latestMonitoringVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Get project system projectId
	project, err := projects.GetProjectByName(client, clusterID, projectName)
	require.NoError(m.T(), err)

	m.project = project
	require.NotEmpty(m.T(), m.project)

	m.chartInstallOptions = &charts.InstallOptions{
		ClusterName: clusterName,
		ClusterID:   clusterID,
		Version:     latestMonitoringVersion,
		ProjectID:   m.project.ID,
	}
	m.chartFeatureOptions = &charts.RancherMonitoringOpts{
		IngressNginx:         true,
		RKEControllerManager: true,
		RKEEtcd:              true,
		RKEProxy:             true,
		RKEScheduler:         true,
	}
}

func (m *MonitoringTestSuite) TestMonitoringChart() {
	subSession := m.session.NewSession()
	defer subSession.Cleanup()

	client, err := m.client.WithSession(subSession)
	require.NoError(m.T(), err)

	m.T().Log("Checking if the monitoring chart is already installed")
	initialMonitoringChart, err := charts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	if !initialMonitoringChart.IsAlreadyInstalled {
		m.T().Log("Installing monitoring chart")
		err = charts.InstallRancherMonitoringChart(client, m.chartInstallOptions, m.chartFeatureOptions)
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)
	}

	paths := []string{alertManagerPath, grafanaPath, prometheusGraphPath, prometheusRulesPath, prometheusTargetsPath}
	for _, path := range paths {
		m.T().Logf("Validating %s is accessible", path)
		result, err := charts.GetChartCaseEndpoint(client, client.RancherConfig.Host, path, true)
		assert.NoError(m.T(), err)
		assert.True(m.T(), result.Ok)
	}

	m.T().Log("Validating all Prometheus active targets are up")
	prometheusTargetsResult, err := checkPrometheusTargets(client)
	assert.NoError(m.T(), err)
	assert.True(m.T(), prometheusTargetsResult)

	m.T().Log("Creating webhook receiver's namespace")
	webhookReceiverNamespace, err := namespaces.CreateNamespace(client, webhookReceiverNamespaceName, "{}", map[string]string{}, map[string]string{}, m.project)
	require.NoError(m.T(), err)

	m.T().Log("Creating alert webhook receiver deployment and its resources")
	alertWebhookReceiverDeployment, err := createAlertWebhookReceiverDeployment(client, m.project.ClusterID, webhookReceiverNamespace.Name, webhookReceiverDeploymentName)
	require.NoError(m.T(), err)
	assert.Equal(m.T(), alertWebhookReceiverDeployment.Name, webhookReceiverDeploymentName)

	m.T().Log("Waiting webhook receiver deployment to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, m.project.ClusterID, webhookReceiverNamespace.Name, metav1.ListOptions{})
	require.NoError(m.T(), err)

	m.T().Log("Creating node port service for webhook receiver deployment")
	serviceSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{
			{
				Name: "port",
				Port: 8080,
			},
		},
		Selector: alertWebhookReceiverDeployment.Spec.Template.Labels,
	}
	webhookReceiverService, err := services.CreateService(client, m.project.ClusterID, webhookReceiverServiceName, webhookReceiverNamespace.Name, serviceSpec)
	require.NoError(m.T(), err)

	// Get a random worker node' public external IP of a specific cluster
	nodeCollection, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": m.project.ClusterID,
	}})
	require.NoError(m.T(), err)
	workerNodePublicIPs := []string{}
	for _, node := range nodeCollection.Data {
		workerNodePublicIPs = append(workerNodePublicIPs, node.Annotations["rke.cattle.io/external-ip"])
	}
	randWorkerNodePublicIP := workerNodePublicIPs[rand.Intn(len(workerNodePublicIPs))]

	// Get URL and string versions of origin with random node' public IP
	hostWithProtocol := fmt.Sprintf("http://%v:%v", randWorkerNodePublicIP, webhookReceiverService.Spec.Ports[0].NodePort)
	urlOfHost, err := url.Parse(hostWithProtocol)
	require.NoError(m.T(), err)

	m.T().Logf("Getting alert manager secret")
	alertManagerSecret, err := secrets.GetSecretByName(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringAlertSecret, metav1.GetOptions{})
	require.NoError(m.T(), err)

	m.T().Logf("Editing alert manager secret receivers")
	encodedAlertConfigWithReceiver, err := editAlertReceiver(alertManagerSecret.Data[secretPath], hostWithProtocol, urlOfHost)
	require.NoError(m.T(), err)

	patchedSecret, err := secrets.PatchSecret(client, m.project.ClusterID, charts.RancherMonitoringAlertSecret, charts.RancherMonitoringNamespace, apimachinerytypes.JSONPatchType, secrets.AddPatchOP, secretPathForPatch, encodedAlertConfigWithReceiver, metav1.PatchOptions{})
	require.NoError(m.T(), err)
	assert.Equal(m.T(), patchedSecret.Name, charts.RancherMonitoringAlertSecret)

	m.T().Logf("Creating prometheus rule")
	err = createPrometheusRule(client, m.project.ClusterID)
	require.NoError(m.T(), err)

	m.T().Logf("Getting alert manager secret")
	alertManagerSecret, err = secrets.GetSecretByName(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringAlertSecret, metav1.GetOptions{})
	require.NoError(m.T(), err)

	m.T().Logf("Editing alert manager secret routes")
	encodedAlertConfigWithRoute, err := editAlertRoute(alertManagerSecret.Data[secretPath], hostWithProtocol, urlOfHost)
	require.NoError(m.T(), err)

	patchedSecret, err = secrets.PatchSecret(client, m.project.ClusterID, charts.RancherMonitoringAlertSecret, charts.RancherMonitoringNamespace, apimachinerytypes.JSONPatchType, secrets.AddPatchOP, secretPathForPatch, encodedAlertConfigWithRoute, metav1.PatchOptions{})
	require.NoError(m.T(), err)
	assert.Equal(m.T(), patchedSecret.Name, charts.RancherMonitoringAlertSecret)

	m.T().Logf("Validating traefik is accessible externally")
	host := fmt.Sprintf("%v:%v", randWorkerNodePublicIP, webhookReceiverService.Spec.Ports[0].NodePort)
	result, err := charts.GetChartCaseEndpoint(client, host, "dashboard", false)
	assert.NoError(m.T(), err)
	assert.True(m.T(), result.Ok)

	m.T().Logf("Validating alertmanager sent alert to webhook receiver")
	err = charts.WatchAndWaitDeploymentForAnnotation(client, m.project.ClusterID, webhookReceiverNamespace.Name, alertWebhookReceiverDeployment.Name, webhookReceiverAnnotationKey, webhookReceiverAnnotationValue)
	require.NoError(m.T(), err)
}

func (m *MonitoringTestSuite) TestUpgradeMonitoringChart() {
	subSession := m.session.NewSession()
	defer subSession.Cleanup()

	client, err := m.client.WithSession(subSession)
	require.NoError(m.T(), err)

	m.T().Log("Checking if the monitoring chart is installed with one of the previous versions")
	initialMonitoringChart, err := charts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Change monitoring install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.RancherMonitoringName)
	require.NoError(m.T(), err)
	require.Greaterf(m.T(), len(versionsList), 2, "There should be at least 2 versions of the monitoring chart")
	versionLatest := versionsList[0]
	versionBeforeLatest := versionsList[1]
	m.chartInstallOptions.Version = versionBeforeLatest

	if initialMonitoringChart.IsAlreadyInstalled && initialMonitoringChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		m.T().Skip("Skipping the upgrade case, monitoring chart is already installed with the latest version")
	}

	if !initialMonitoringChart.IsAlreadyInstalled {
		m.T().Log("Installing monitoring chart with the last but one version")
		err = charts.InstallRancherMonitoringChart(client, m.chartInstallOptions, m.chartFeatureOptions)
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)
	}

	monitoringChartPreUpgrade, err := charts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Validate current version of rancher monitoring is one of the versions before latest
	chartVersionPreUpgrade := monitoringChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	assert.Contains(m.T(), versionsList[1:], chartVersionPreUpgrade)

	m.chartInstallOptions.Version, err = client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	m.T().Log("Upgrading monitoring chart with the latest version")
	err = charts.UpgradeRancherMonitoringChart(client, m.chartInstallOptions, m.chartFeatureOptions)
	require.NoError(m.T(), err)

	monitoringChartPostUpgrade, err := charts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Compare rancher monitoring versions
	chartVersionPostUpgrade := monitoringChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	assert.Equal(m.T(), m.chartInstallOptions.Version, chartVersionPostUpgrade)
}

func (m *MonitoringTestSuite) TestVerifyNoPendingHelmOp() {
	subSession := m.session.NewSession()
	defer subSession.Cleanup()

	client, err := m.client.WithSession(subSession)
	require.NoError(m.T(), err)

	m.T().Log("Asserting that all helm-operation-xxxx pods terminated")
	done := workloads.WaitPodTerminated(client, m.project.ClusterID, "helm-operation-")
	require.True(m.T(), done)
}

func TestMonitoringTestSuite(t *testing.T) {
	suite.Run(t, new(MonitoringTestSuite))
}
