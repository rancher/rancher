//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	"github.com/rancher/rancher/tests/v2/actions/observability"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/pkg/config"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

type StackStateInstallTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	cluster                       *clusters.ClusterMeta
	projectID                     string
	catalogClient                 *catalog.Client
	stackstateChartInstallOptions *charts.InstallOptions
	stackstateConfigs             *observability.StackStateConfig
}

const (
	observabilityChartURL  = "https://charts.rancher.com/server-charts/prime/suse-observability"
	observabilityChartName = "suse-observability"
)

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

	ssi.catalogClient, err = ssi.client.GetClusterCatalogClient(ssi.cluster.ID)
	require.NoError(ssi.T(), err)

	//ssi.Require().NoError(ssi.pollUntilDownloaded("suse-observability", metav1.Time{}))

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

	var stackstateConfigs observability.StackStateConfig
	config.LoadConfig(stackStateConfigFileKey, &stackstateConfigs)
	ssi.stackstateConfigs = &stackstateConfigs

	// Install StackState Chart Repo
	err = charts.CreateClusterRepo(ssi.client, ssi.catalogClient, observabilityChartName, observabilityChartURL)
	require.NoError(ssi.T(), err)

	latestSSVersion, err := ssi.client.Catalog.GetLatestChartVersion(charts.StackStateChartRepo, observabilityChartName)

	ssi.stackstateChartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestSSVersion,
		ProjectID: ssi.projectID,
	}
}

func (ssi *StackStateInstallTestSuite) TestInstallStackstate() {
	subsession := ssi.session.NewSession()
	defer subsession.Cleanup()

	ssi.Run("Install SUSE Observability Chart", func() {
		// First create the cluster repo
		////err := charts.CreateClusterRepo(ssi.client, ssi.catalogClient, observabilityChartName, observabilityChartURL)
		////require.NoError(ssi.T(), err)
		//
		//// Get the latest version of the chart
		//latestVersion, err := ssi.client.Catalog.GetLatestChartVersion(observabilityChartName, observabilityChartName)
		//require.NoError(ssi.T(), err)
		//
		//// Create install options
		//installOptions := &charts.InstallOptions{
		//	Cluster:   ssi.cluster,
		//	Version:   latestVersion,
		//	ProjectID: ssi.projectID,
		//}

		systemProject, err := projects.GetProjectByName(ssi.client, ssi.cluster.ID, systemProject)
		require.NoError(ssi.T(), err)
		require.NotNil(ssi.T(), systemProject.ID, "System project is nil.")
		systemProjectID := strings.Split(systemProject.ID, ":")[1]

		// Install the chart
		err = charts.InstallStackStateChart(ssi.client, ssi.stackstateChartInstallOptions, ssi.stackstateConfigs, systemProjectID)
		require.NoError(ssi.T(), err)

		// Register cleanup to uninstall the chart after test
		//ssi.session.RegisterCleanupFunc(func() error {
		//	uninstallAction := charts.NewChartUninstallAction()
		//	return ssi.catalogClient.UninstallChart(uninstallAction)
		//})
	})
}

func TestStackStateInstallTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateInstallTestSuite))
}
