//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package observability

import (
	"context"
	"strings"

	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/uiplugins"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	project                 = "management.cattle.io.project"
	rancherPartnerCharts    = "rancher-partner-charts"
	systemProject           = "System"
	localCluster            = "local"
	stackStateConfigFileKey = "stackstateConfigs"
	uiExtensionsRepo        = "https://github.com/rancher/ui-plugin-charts"
	uiGitBranch             = "main"
	rancherUIPlugins        = "rancher-ui-plugins"
)

type StackStateTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	cluster                       *clusters.ClusterMeta
	projectID                     string
	stackstateAgentInstallOptions *charts.InstallOptions
	stackstateConfigs             *observability.StackStateConfig
}

func (ss *StackStateTestSuite) TearDownSuite() {
	ss.session.Cleanup()
}

func (ss *StackStateTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ss.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ss.T(), err)

	ss.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ss.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(ss.client, clusterName)
	require.NoError(ss.T(), err)
	ss.cluster = cluster

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(ss.T(), err)
	ss.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackstateNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(ss.T(), err)

	_, err = ss.client.Catalog.ClusterRepos().Get(context.TODO(), rancherUIPlugins, meta.GetOptions{})

	if k8sErrors.IsNotFound(err) {
		err = uiplugins.CreateExtensionsRepo(ss.client, rancherUIPlugins, uiExtensionsRepo, uiGitBranch)
		log.Info("Created an extensions repo for ui plugins.")
	}
	require.NoError(ss.T(), err)

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	ss.stackstateConfigs = &stackstateConfigs

	err = observability.WhitelistStackstateDomains(ss.client, []string{ss.stackstateConfigs.Url})
	require.NoError(ss.T(), err)
	log.Info("Node driver installed with stackstate extensions ui to whitelist stackstate URL")

	crdsExists, err := ss.client.Steve.SteveType(observability.ApiExtenisonsCRD).ByID(observability.ObservabilitySteveType)
	if crdsExists == nil && strings.Contains(err.Error(), "Not Found") {
		err = observability.InstallStackstateCRD(ss.client)
		log.Info("Installed stackstate CRD")
	}
	require.NoError(ss.T(), err)

	client, err = client.ReLogin()
	require.NoError(ss.T(), err)

	initialStackstateExtension, err := extencharts.GetChartStatus(client, localCluster, charts.StackstateExtensionNamespace, charts.StackstateExtensionsName)
	require.NoError(ss.T(), err)

	if !initialStackstateExtension.IsAlreadyInstalled {
		latestUIPluginVersion, err := ss.client.Catalog.GetLatestChartVersion(charts.StackstateExtensionsName, charts.UIPluginName)
		require.NoError(ss.T(), err)

		extensionOptions := &uiplugins.ExtensionOptions{
			ChartName:   charts.StackstateExtensionsName,
			ReleaseName: charts.StackstateExtensionsName,
			Version:     latestUIPluginVersion,
		}

		err = uiplugins.InstallObservabilityUiPlugin(client, extensionOptions)
		require.NoError(ss.T(), err)
		log.Info("Installed stackstate ui extensions")
	}

	steveAdminClient, err := client.Steve.ProxyDownstream(localCluster)
	require.NoError(ss.T(), err)

	crdConfig := observability.NewStackstateCRDConfiguration(charts.StackstateNamespace, observability.StackstateName, ss.stackstateConfigs)
	crd, err := steveAdminClient.SteveType(charts.StackstateCRD).Create(crdConfig)
	require.NoError(ss.T(), err)
	log.Info("Created stackstate ui extensions configuration")

	_, err = steveAdminClient.SteveType(charts.StackstateCRD).ByID(crd.ID)
	require.NoError(ss.T(), err)

	latestSSVersion, err := ss.client.Catalog.GetLatestChartVersion(charts.StackstateK8sAgent, rancherPartnerCharts)
	require.NoError(ss.T(), err)

	ss.stackstateAgentInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: project.ID,
	}
}

func (ss *StackStateTestSuite) TestStackStateAgentChart() {
	subSession := ss.session.NewSession()
	defer subSession.Cleanup()

	client, err := ss.client.WithSession(subSession)
	require.NoError(ss.T(), err)

	initialStackstateAgent, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
	require.NoError(ss.T(), err)

	if initialStackstateAgent.IsAlreadyInstalled {
		ss.T().Skip("Stack state agent is already installed, skipping the tests.")
	}

	log.Info("Installing stack state agent on the provided cluster")

	systemProject, err := projects.GetProjectByName(client, ss.cluster.ID, systemProject)
	require.NoError(ss.T(), err)
	require.NotNil(ss.T(), systemProject.ID)
	systemProjectID := strings.Split(systemProject.ID, ":")[1]

	ss.Run(charts.StackstateK8sAgent+" "+ss.stackstateAgentInstallOptions.Version, func() {
		err = charts.InstallStackstateAgentChart(ss.client, ss.stackstateAgentInstallOptions, ss.stackstateConfigs, systemProjectID)
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(ss.client, ss.client.RancherConfig.ClusterName, fleet.Namespace)
		if clusterObject != nil {
			status := &provv1.ClusterStatus{}
			err := steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(ss.T(), err)

			podErrors := pods.StatusPods(client, status.ClusterName)
			require.Empty(ss.T(), podErrors)
		}
	})
}

func (ss *StackStateTestSuite) TestUpgradeStackstateAgentChart() {
	subSession := ss.session.NewSession()
	defer subSession.Cleanup()

	client, err := ss.client.WithSession(subSession)
	require.NoError(ss.T(), err)

	versionsList, err := client.Catalog.GetListChartVersions(charts.StackstateK8sAgent, rancherPartnerCharts)
	require.NoError(ss.T(), err)

	if len(versionsList) < 2 {
		ss.T().Skip("Skipping the upgrade case, only one version of stackstate agent chart is available")
	}

	versionLatest := versionsList[0]
	versionBeforeLatest := versionsList[1]
	ss.stackstateAgentInstallOptions.Version = versionBeforeLatest

	ss.Run(charts.StackstateK8sAgent+" "+ss.stackstateAgentInstallOptions.Version, func() {
		initialStackstateAgent, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
		require.NoError(ss.T(), err)

		if initialStackstateAgent.IsAlreadyInstalled {
			ss.T().Skip("Skipping the upgrade case, stackstate agent chart is already installed.")
		}

		systemProject, err := projects.GetProjectByName(client, ss.cluster.ID, systemProject)
		require.NoError(ss.T(), err)
		require.NotNil(ss.T(), systemProject.ID)
		systemProjectID := strings.Split(systemProject.ID, ":")[1]

		ss.T().Logf("Installing stackstate agent chart with the version before the latest version %v", ss.stackstateAgentInstallOptions.Version)
		err = charts.InstallStackstateAgentChart(client, ss.stackstateAgentInstallOptions, ss.stackstateConfigs, systemProjectID)
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		stackstateAgentChartPreUpgrade, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
		require.NoError(ss.T(), err)

		// Validate current version of stackstate agent is one of the versions before latest
		chartVersionPreUpgrade := stackstateAgentChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
		require.Contains(ss.T(), versionsList[1:], chartVersionPreUpgrade)

		ss.stackstateAgentInstallOptions.Version = versionLatest

		ss.T().Logf("Upgrading stackstate agent chart to the latest version %v", ss.stackstateAgentInstallOptions.Version)
		err = charts.UpgradeStackstateAgentChart(client, ss.stackstateAgentInstallOptions, ss.stackstateConfigs, systemProject.ID)
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		clusterObject, _, err := extensionscluster.GetProvisioningClusterByName(ss.client, ss.client.RancherConfig.ClusterName, fleet.Namespace)
		require.NoError(ss.T(), err)

		var clusterName string

		if clusterObject != nil {
			status := &provv1.ClusterStatus{}
			err := steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(ss.T(), err)
			clusterName = status.ClusterName
		} else {
			clusterName, err = extensionscluster.GetClusterIDByName(ss.client, ss.client.RancherConfig.ClusterName)
			require.NoError(ss.T(), err)
		}
		podErrors := pods.StatusPods(client, clusterName)
		require.Empty(ss.T(), podErrors)

		stackstateAgentChartPostUpgrade, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
		require.NoError(ss.T(), err)

		ss.T().Log("Comparing installed and desired stackstate agent versions")
		chartVersionPostUpgrade := stackstateAgentChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
		require.Equal(ss.T(), ss.stackstateAgentInstallOptions.Version, chartVersionPostUpgrade)
	})
}

func (ss *StackStateTestSuite) TestDynamicUpgradeStackstateAgentChart() {

	subSession := ss.session.NewSession()
	defer subSession.Cleanup()

	client, err := ss.client.WithSession(subSession)
	require.NoError(ss.T(), err)

	versionToUpgrade := ss.stackstateConfigs.UpgradeVersion
	if versionToUpgrade == "" {
		ss.T().Skip("Skipping the test as no user version provided")
	}

	ss.Run(charts.StackstateK8sAgent+" "+ss.stackstateAgentInstallOptions.Version, func() {
		initialStackstateAgent, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
		require.NoError(ss.T(), err)

		if !initialStackstateAgent.IsAlreadyInstalled || initialStackstateAgent.ChartDetails.Spec.Chart.Metadata.Version == versionToUpgrade {
			ss.T().Skip("Skipping the test, as stackstate agent chart is already installed with the provided version or stackstate agent is not installed.")
		}

		ss.stackstateAgentInstallOptions.Version = ss.stackstateConfigs.UpgradeVersion
		require.NoError(ss.T(), err)

		ss.T().Logf("Upgrading stackstate agent chart to the user provided version %v", ss.stackstateAgentInstallOptions.Version)
		err = charts.UpgradeStackstateAgentChart(client, ss.stackstateAgentInstallOptions, ss.stackstateConfigs, systemProject)
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		ss.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, ss.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ss.T(), err)

		clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(ss.client, ss.client.RancherConfig.ClusterName, fleet.Namespace)

		var clusterName string
		if clusterObject != nil {
			status := &provv1.ClusterStatus{}
			err := steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(ss.T(), err)
			clusterName = status.ClusterName
		} else {
			clusterName, err = extensionscluster.GetClusterIDByName(ss.client, ss.client.RancherConfig.ClusterName)
			require.NoError(ss.T(), err)
		}
		podErrors := pods.StatusPods(client, clusterName)
		require.Empty(ss.T(), podErrors)

		stackstateAgentChartPostUpgrade, err := extencharts.GetChartStatus(client, ss.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
		require.NoError(ss.T(), err)

		ss.T().Log("Comparing installed and desired stackstate agent versions")
		chartVersionPostUpgrade := stackstateAgentChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
		require.Equal(ss.T(), ss.stackstateAgentInstallOptions.Version, chartVersionPostUpgrade)
	})
}

func TestStackStateTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateTestSuite))
}
