//go:build (validation || infra.rke1 || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !sanity && !extended

package charts

import (
	"fmt"
	"math/rand"
	"net/url"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/secrets"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/ingresses"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	testSession := session.NewSession()
	m.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(m.T(), err)

	m.client = client

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(m.T(), clusterName, "Cluster name to install is not set")

	// Get cluster meta
	cluster, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(m.T(), err)

	// Change alert manager and grafana paths if it's not local cluster
	if !cluster.IsLocal {
		alertManagerPath = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, alertManagerPath)
		grafanaPath = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, grafanaPath)
		prometheusTargetsPathAPI = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, prometheusTargetsPathAPI)
	}

	// Change prometheus paths to use the clusterID
	prometheusGraphPath = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, prometheusGraphPath)
	prometheusRulesPath = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, prometheusRulesPath)
	prometheusTargetsPath = fmt.Sprintf("k8s/clusters/%s/%s", cluster.ID, prometheusTargetsPath)

	// Get latest versions of the monitoring chart
	latestMonitoringVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName, catalog.RancherChartRepo)
	require.NoError(m.T(), err)

	// Get project system projectId
	project, err := projects.GetProjectByName(client, cluster.ID, projectName)
	require.NoError(m.T(), err)

	m.project = project
	require.NotEmpty(m.T(), m.project)

	m.chartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestMonitoringVersion,
		ProjectID: m.project.ID,
	}
	m.chartFeatureOptions = &charts.RancherMonitoringOpts{
		IngressNginx:      true,
		ControllerManager: true,
		Etcd:              true,
		Proxy:             true,
		Scheduler:         true,
	}
}

func (m *MonitoringTestSuite) TestMonitoringChart() {
	subSession := m.session.NewSession()
	defer subSession.Cleanup()

	client, err := m.client.WithSession(subSession)
	require.NoError(m.T(), err)

	steveclient, err := client.Steve.ProxyDownstream(m.project.ClusterID)
	require.NoError(m.T(), err)

	m.T().Log("Checking if the monitoring chart is already installed")
	initialMonitoringChart, err := extencharts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	if !initialMonitoringChart.IsAlreadyInstalled {
		m.T().Log("Installing monitoring chart")
		err = charts.InstallRancherMonitoringChart(client, m.chartInstallOptions, m.chartFeatureOptions)
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = extencharts.WatchAndWaitStatefulSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)
	}

	paths := []string{alertManagerPath, grafanaPath, prometheusGraphPath, prometheusRulesPath, prometheusTargetsPath}
	for _, path := range paths {
		m.T().Logf("Validating %s is accessible", path)
		result, err := ingresses.IsIngressExternallyAccessible(client, client.RancherConfig.Host, path, true)
		assert.NoError(m.T(), err)
		assert.True(m.T(), result)
	}

	m.T().Log("Validating all Prometheus active targets are up")
	prometheusTargetsResult, err := checkPrometheusTargets(client)
	assert.NoError(m.T(), err)
	assert.True(m.T(), prometheusTargetsResult)

	m.T().Log("Creating webhook receiver's namespace")
	webhookReceiverNamespace, err := namespaces.CreateNamespace(client, webhookReceiverNamespaceName, "{}", map[string]string{}, map[string]string{}, m.project)
	require.NoError(m.T(), err)

	m.T().Log("Creating alert webhook receiver deployment and its resources")
	alertWebhookReceiverDeploymentResp, err := createAlertWebhookReceiverDeployment(client, m.project.ClusterID, webhookReceiverNamespace.Name, webhookReceiverDeploymentName)
	require.NoError(m.T(), err)
	assert.Equal(m.T(), alertWebhookReceiverDeploymentResp.Name, webhookReceiverDeploymentName)

	m.T().Log("Waiting webhook receiver deployment to have expected number of available replicas")
	err = extencharts.WatchAndWaitDeployments(client, m.project.ClusterID, webhookReceiverNamespace.Name, metav1.ListOptions{})
	require.NoError(m.T(), err)

	alertWebhookReceiverDeploymentSpec := &appv1.DeploymentSpec{}
	err = v1.ConvertToK8sType(alertWebhookReceiverDeploymentResp.Spec, alertWebhookReceiverDeploymentSpec)
	require.NoError(m.T(), err)

	m.T().Log("Creating node port service for webhook receiver deployment")
	webhookServiceTemplate := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookReceiverServiceName,
			Namespace: webhookReceiverNamespace.Name,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name: "port",
					Port: 8080,
				},
			},
			Selector: alertWebhookReceiverDeploymentSpec.Template.Labels,
		},
	}
	webhookReceiverServiceResp, err := steveclient.SteveType(services.ServiceSteveType).Create(webhookServiceTemplate)
	require.NoError(m.T(), err)

	webhookReceiverServiceSpec := &corev1.ServiceSpec{}
	err = v1.ConvertToK8sType(webhookReceiverServiceResp.Spec, webhookReceiverServiceSpec)
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
	hostWithProtocol := fmt.Sprintf("http://%v:%v", randWorkerNodePublicIP, webhookReceiverServiceSpec.Ports[0].NodePort)
	urlOfHost, err := url.Parse(hostWithProtocol)
	require.NoError(m.T(), err)

	m.T().Logf("Getting alert manager secret to edit receiver")
	alertManagerSecretResp, err := steveclient.SteveType(secrets.SecretSteveType).ByID(alertManagerSecretID)
	require.NoError(m.T(), err)

	alertManagerSecret := &corev1.Secret{}
	err = v1.ConvertToK8sType(alertManagerSecretResp.JSONResp, alertManagerSecret)
	require.NoError(m.T(), err)

	m.T().Logf("Editing alert manager secret receivers")
	encodedAlertConfigWithReceiver, err := editAlertReceiver(alertManagerSecret.Data[secretPath], urlOfHost)
	require.NoError(m.T(), err)

	alertManagerSecret.Data[secretPath] = encodedAlertConfigWithReceiver

	editedReceiverSecretResp, err := steveclient.SteveType(secrets.SecretSteveType).Update(alertManagerSecretResp, alertManagerSecret)
	require.NoError(m.T(), err)
	assert.Equal(m.T(), editedReceiverSecretResp.Name, charts.RancherMonitoringAlertSecret)

	m.T().Logf("Creating prometheus rule")
	err = createPrometheusRule(client, m.project.ClusterID)
	require.NoError(m.T(), err)

	m.T().Logf("Getting alert manager secret to edit routes")
	alertManagerSecretResp, err = steveclient.SteveType(secrets.SecretSteveType).ByID(alertManagerSecretID)
	require.NoError(m.T(), err)

	err = v1.ConvertToK8sType(alertManagerSecretResp.JSONResp, alertManagerSecret)
	require.NoError(m.T(), err)

	m.T().Logf("Editing alert manager secret routes")
	encodedAlertConfigWithRoute, err := editAlertRoute(alertManagerSecret.Data[secretPath])
	require.NoError(m.T(), err)

	alertManagerSecret.Data[secretPath] = encodedAlertConfigWithRoute

	editedRouteSecretResp, err := steveclient.SteveType(secrets.SecretSteveType).Update(alertManagerSecretResp, alertManagerSecret)
	require.NoError(m.T(), err)
	assert.Equal(m.T(), editedRouteSecretResp.Name, charts.RancherMonitoringAlertSecret)

	m.T().Logf("Validating traefik is accessible externally")
	host := fmt.Sprintf("%v:%v", randWorkerNodePublicIP, webhookReceiverServiceSpec.Ports[0].NodePort)
	result, err := ingresses.IsIngressExternallyAccessible(client, host, "dashboard", false)
	assert.NoError(m.T(), err)
	assert.True(m.T(), result)

	m.T().Logf("Validating alertmanager sent alert to webhook receiver")
	err = extencharts.WatchAndWaitDeploymentForAnnotation(client, m.project.ClusterID, webhookReceiverNamespace.Name, alertWebhookReceiverDeploymentResp.Name, webhookReceiverAnnotationKey, webhookReceiverAnnotationValue)
	require.NoError(m.T(), err)
}

func (m *MonitoringTestSuite) TestUpgradeMonitoringChart() {
	subSession := m.session.NewSession()
	defer subSession.Cleanup()

	client, err := m.client.WithSession(subSession)
	require.NoError(m.T(), err)

	m.T().Log("Checking if the monitoring chart is installed with one of the previous versions")
	initialMonitoringChart, err := extencharts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Change monitoring install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.RancherMonitoringName, catalog.RancherChartRepo)
	require.NoError(m.T(), err)
	require.Greaterf(m.T(), len(versionsList), 1, "There should be at least 2 versions of the monitoring chart")
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
		err = extencharts.WatchAndWaitDeployments(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)

		m.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = extencharts.WatchAndWaitStatefulSets(client, m.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(m.T(), err)
	}

	monitoringChartPreUpgrade, err := extencharts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Validate current version of rancher monitoring is one of the versions before latest
	chartVersionPreUpgrade := monitoringChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	assert.Contains(m.T(), versionsList[1:], chartVersionPreUpgrade)

	m.chartInstallOptions.Version, err = client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName, catalog.RancherChartRepo)
	require.NoError(m.T(), err)

	m.T().Log("Upgrading monitoring chart with the latest version")
	err = charts.UpgradeRancherMonitoringChart(client, m.chartInstallOptions, m.chartFeatureOptions)
	require.NoError(m.T(), err)

	monitoringChartPostUpgrade, err := extencharts.GetChartStatus(client, m.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(m.T(), err)

	// Compare rancher monitoring versions
	chartVersionPostUpgrade := monitoringChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	assert.Equal(m.T(), m.chartInstallOptions.Version, chartVersionPostUpgrade)
}

func TestMonitoringTestSuite(t *testing.T) {
	suite.Run(t, new(MonitoringTestSuite))
}
