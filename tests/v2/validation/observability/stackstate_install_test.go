//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
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
	"strings"
	"testing"
)

type StackStateInstallTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	cluster                       *clusters.ClusterMeta
	projectID                     string
	stackstateAgentInstallOptions *charts.InstallOptions
	stackstateConfigs             *observability.StackStateConfig
}

func (ssi *StackStateInstallTestSuite) TearDownSuite() {
	ssi.session.Cleanup()
}

func (ssi *StackStateInstallTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ssi.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ssi.T(), err)

	ssi.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ssi.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(ssi.client, clusterName)
	require.NoError(ssi.T(), err)
	ssi.cluster = cluster

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(ssi.T(), err)
	ssi.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackstateNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(ssi.T(), err)

	//_, err = ssi.client.Catalog.ClusterRepos().Get(context.TODO(), rancherUIPlugins, meta.GetOptions{})

	//if k8sErrors.IsNotFound(err) {
	//	err = uiplugins.CreateExtensionsRepo(ssi.client, rancherUIPlugins, uiExtensionsRepo, uiGitBranch)
	//	log.Info("Created an extensions repo for ui plugins.")
	//}
	//require.NoError(ssi.T(), err)

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	ssi.stackstateConfigs = &stackstateConfigs

	err = observability.WhitelistStackstateDomains(ssi.client, []string{ssi.stackstateConfigs.Url})
	require.NoError(ssi.T(), err)
	log.Info("Node driver installed with stackstate extensions ui to whitelist stackstate URL")

	crdsExists, err := ssi.client.Steve.SteveType(observability.ApiExtenisonsCRD).ByID(observability.ObservabilitySteveType)
	if crdsExists == nil && err != nil && strings.Contains(err.Error(), "Not Found") {
		err = observability.InstallStackstateCRD(ssi.client)
		log.Info("Installed stackstate CRD")
	}
	require.NoError(ssi.T(), err)

	client, err = client.ReLogin()
	require.NoError(ssi.T(), err)

	initialStackstateExtension, err := extencharts.GetChartStatus(client, localCluster, charts.StackstateExtensionNamespace, charts.StackstateExtensionsName)
	require.NoError(ssi.T(), err)

	if !initialStackstateExtension.IsAlreadyInstalled {
		latestUIPluginVersion, err := ssi.client.Catalog.GetLatestChartVersion(charts.StackstateExtensionsName, charts.UIPluginName)
		require.NoError(ssi.T(), err)

		extensionOptions := &uiplugins.ExtensionOptions{
			ChartName:   charts.StackstateExtensionsName,
			ReleaseName: charts.StackstateExtensionsName,
			Version:     latestUIPluginVersion,
		}

		err = uiplugins.InstallObservabilityUiPlugin(client, extensionOptions)
		require.NoError(ssi.T(), err)
		log.Info("Installed stackstate ui extensions")
	}

	steveAdminClient, err := client.Steve.ProxyDownstream(localCluster)
	require.NoError(ssi.T(), err)

	crdConfig := observability.NewStackstateCRDConfiguration(charts.StackstateNamespace, observability.StackstateName, ssi.stackstateConfigs)
	crd, err := steveAdminClient.SteveType(charts.StackstateCRD).Create(crdConfig)
	require.NoError(ssi.T(), err)
	log.Info("Created stackstate ui extensions configuration")

	_, err = steveAdminClient.SteveType(charts.StackstateCRD).ByID(crd.ID)
	require.NoError(ssi.T(), err)

	latestSSVersion, err := ssi.client.Catalog.GetLatestChartVersion(charts.StackstateK8sAgent, rancherPartnerCharts)
	require.NoError(ssi.T(), err)

	ssi.stackstateAgentInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: project.ID,
	}
}

func (ssi *StackStateInstallTestSuite) TestStackStateAgentChart() {
	subSession := ssi.session.NewSession()
	defer subSession.Cleanup()

	client, err := ssi.client.WithSession(subSession)
	require.NoError(ssi.T(), err)

	initialStackstateAgent, err := extencharts.GetChartStatus(client, ssi.cluster.ID, charts.StackstateNamespace, charts.StackstateK8sAgent)
	require.NoError(ssi.T(), err)

	if initialStackstateAgent.IsAlreadyInstalled {
		ssi.T().Skip("Stack state agent is already installed, skipping the tests.")
	}

	log.Info("Installing stack state agent on the provided cluster")

	systemProject, err := projects.GetProjectByName(client, ssi.cluster.ID, systemProject)
	require.NoError(ssi.T(), err)
	require.NotNil(ssi.T(), systemProject.ID)
	systemProjectID := strings.Split(systemProject.ID, ":")[1]

	ssi.Run(charts.StackstateK8sAgent+" "+ssi.stackstateAgentInstallOptions.Version, func() {
		err = charts.InstallStackstateAgentChart(ssi.client, ssi.stackstateAgentInstallOptions, ssi.stackstateConfigs, systemProjectID)
		require.NoError(ssi.T(), err)

		ssi.T().Log("Verifying the deployments of stackstate agent chart to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, ssi.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ssi.T(), err)

		ssi.T().Log("Verifying the daemonsets of stackstate agent chart to have expected number of available replicas nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, ssi.cluster.ID, charts.StackstateNamespace, meta.ListOptions{})
		require.NoError(ssi.T(), err)

		clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(ssi.client, ssi.client.RancherConfig.ClusterName, fleet.Namespace)
		if clusterObject != nil {
			status := &provv1.ClusterStatus{}
			err := steveV1.ConvertToK8sType(clusterObject.Status, status)
			require.NoError(ssi.T(), err)

			podErrors := pods.StatusPods(client, status.ClusterName)
			require.Empty(ssi.T(), podErrors)
		}
	})
}

func (ssi *StackStateInstallTestSuite) TestStackStateInstall() {
	subsession := ssi.session.NewSession()
	defer subsession.Cleanup()
	//client, err := ssi.client.WithSession(subsession)
	//require.NoError(ssi.T(), err)

	ssi.Run("Install Stackstate", func() {

		err := charts.InstallStackStateWithHelm()
		require.NoError(ssi.T(), err)
	})

}

func TestStackStateInstallTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateInstallTestSuite))
}
