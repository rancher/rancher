//go:build (validation || infra.rke1 || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !sanity && !extended

package charts

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/registries"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InstallationTestSuite struct {
	suite.Suite
	client          *rancher.Client
	session         *session.Session
	project         *management.Project
	cluster         *clusters.ClusterMeta
	registrySetting *management.Setting
}

func (i *InstallationTestSuite) TearDownSuite() {
	i.session.Cleanup()
}

func (i *InstallationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	i.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(i.T(), err)

	i.client = client

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(i.T(), clusterName, "Cluster name to install is not set")

	// Get cluster meta
	cluster, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(i.T(), err)

	i.cluster = cluster

	// Get Server and Registry Setting Values
	i.registrySetting, err = client.Management.Setting.ByID("system-default-registry")
	require.NoError(i.T(), err)

	// Create project
	projectConfig := &management.Project{
		ClusterID: cluster.ID,
		Name:      exampleAppProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(i.T(), err)
	require.Equal(i.T(), createdProject.Name, exampleAppProjectName)
	i.project = createdProject
}

func (i *InstallationTestSuite) TestInstallMonitoringChart() {
	client, err := i.client.WithSession(i.session)
	require.NoError(i.T(), err)

	i.T().Log("Checking if the monitoring chart is already installed")
	initialMonitoringChart, err := extencharts.GetChartStatus(client, i.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(i.T(), err)

	if !initialMonitoringChart.IsAlreadyInstalled {
		// Get latest versions of monitoring
		latestMonitoringVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName, catalog.RancherChartRepo)
		require.NoError(i.T(), err)

		monitoringInstOpts := &charts.InstallOptions{
			Cluster:   i.cluster,
			Version:   latestMonitoringVersion,
			ProjectID: i.project.ID,
		}

		monitoringOpts := &charts.RancherMonitoringOpts{
			IngressNginx:      true,
			ControllerManager: true,
			Etcd:              true,
			Proxy:             true,
			Scheduler:         true,
		}

		i.T().Logf("Installing monitoring chart with the latest version in cluster [%v] with version [%v]", i.cluster.Name, latestMonitoringVersion)
		err = charts.InstallRancherMonitoringChart(client, monitoringInstOpts, monitoringOpts)
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = extencharts.WatchAndWaitStatefulSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)
	}

	isUsingRegistry, err := registries.CheckAllClusterPodsForRegistryPrefix(client, i.cluster.ID, i.registrySetting.Value)
	require.NoError(i.T(), err)
	assert.Truef(i.T(), isUsingRegistry, "Checking if using correct registry prefix")
}

func (i *InstallationTestSuite) TestInstallAlertingChart() {
	i.TestInstallMonitoringChart()

	client, err := i.client.WithSession(i.session)
	require.NoError(i.T(), err)

	alertingChart, err := extencharts.GetChartStatus(client, i.project.ClusterID, charts.RancherAlertingNamespace, charts.RancherAlertingName)
	require.NoError(i.T(), err)

	if !alertingChart.IsAlreadyInstalled {
		// Get latest versions of alerting
		latestAlertingVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherAlertingName, catalog.RancherChartRepo)
		require.NoError(i.T(), err)

		alertingChartInstallOption := &charts.InstallOptions{
			Cluster:   i.cluster,
			Version:   latestAlertingVersion,
			ProjectID: i.project.ID,
		}

		alertingFeatureOption := &charts.RancherAlertingOpts{
			SMS:   true,
			Teams: true,
		}

		i.T().Logf("Installing alerting chart with the latest version in cluster [%v] with version [%v]", i.cluster.Name, latestAlertingVersion)
		err = charts.InstallRancherAlertingChart(client, alertingChartInstallOption, alertingFeatureOption)
		require.NoError(i.T(), err)
	}

	isUsingRegistry, err := registries.CheckAllClusterPodsForRegistryPrefix(client, i.cluster.ID, i.registrySetting.Value)
	require.NoError(i.T(), err)
	assert.Truef(i.T(), isUsingRegistry, "Checking if using correct registry prefix")
}

func (i *InstallationTestSuite) TestInstallLoggingChart() {
	client, err := i.client.WithSession(i.session)
	require.NoError(i.T(), err)

	loggingChart, err := extencharts.GetChartStatus(client, i.project.ClusterID, charts.RancherLoggingNamespace, charts.RancherLoggingName)
	require.NoError(i.T(), err)

	if !loggingChart.IsAlreadyInstalled {
		// Get latest versions of logging
		latestLoggingVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherLoggingName, catalog.RancherChartRepo)
		require.NoError(i.T(), err)

		loggingChartInstallOption := &charts.InstallOptions{
			Cluster:   i.cluster,
			Version:   latestLoggingVersion,
			ProjectID: i.project.ID,
		}

		loggingChartFeatureOption := &charts.RancherLoggingOpts{
			AdditionalLoggingSources: true,
		}

		i.T().Logf("Installing logging chart with the latest version in cluster [%v] with version [%v]", i.cluster.Name, latestLoggingVersion)
		err = charts.InstallRancherLoggingChart(client, loggingChartInstallOption, loggingChartFeatureOption)
		require.NoError(i.T(), err)
	}

	isUsingRegistry, err := registries.CheckAllClusterPodsForRegistryPrefix(client, i.cluster.ID, i.registrySetting.Value)
	require.NoError(i.T(), err)
	assert.Truef(i.T(), isUsingRegistry, "Checking if using correct registry prefix")
}

func (i *InstallationTestSuite) TestInstallIstioChart() {
	i.TestInstallMonitoringChart()

	client, err := i.client.WithSession(i.session)
	require.NoError(i.T(), err)

	istioChart, err := extencharts.GetChartStatus(client, i.project.ClusterID, charts.RancherIstioNamespace, charts.RancherIstioName)
	require.NoError(i.T(), err)

	if !istioChart.IsAlreadyInstalled {
		// Get latest versions of logging
		latestIstioVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherIstioName, catalog.RancherChartRepo)

		require.NoError(i.T(), err)

		istioChartInstallOption := &charts.InstallOptions{
			Cluster:   i.cluster,
			Version:   latestIstioVersion,
			ProjectID: i.project.ID,
		}

		istioChartFeatureOption := &charts.RancherIstioOpts{
			IngressGateways: true,
			EgressGateways:  false,
			Pilot:           true,
			Telemetry:       true,
			Kiali:           true,
			Tracing:         true,
			CNI:             false,
		}

		i.T().Logf("Installing istio chart with the latest version in cluster [%v] with version [%v]", i.cluster.Name, latestIstioVersion)
		err = charts.InstallRancherIstioChart(client, istioChartInstallOption, istioChartFeatureOption)
		require.NoError(i.T(), err)
	}

	isUsingRegistry, err := registries.CheckAllClusterPodsForRegistryPrefix(client, i.cluster.ID, i.registrySetting.Value)
	require.NoError(i.T(), err)
	assert.Truef(i.T(), isUsingRegistry, "Checking if using correct registry prefix")
}

func TestInstallationTestSuite(t *testing.T) {
	suite.Run(t, new(InstallationTestSuite))
}
