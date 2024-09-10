//go:build (validation || infra.rke1 || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !sanity && !extended

package charts

import (
	"os"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GateKeeperTestSuite struct {
	suite.Suite
	client                        *rancher.Client
	session                       *session.Session
	project                       *management.Project
	gatekeeperChartInstallOptions *charts.InstallOptions
}

func (g *GateKeeperTestSuite) TearDownSuite() {
	g.session.Cleanup()
}

func (g *GateKeeperTestSuite) SetupSuite() {
	testSession := session.NewSession()
	g.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(g.T(), err)

	g.client = client

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(g.T(), clusterName, "Cluster name to install is not set")

	// Get cluster meta
	cluster, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(g.T(), err)

	// get latest version of gatekeeper chart
	latestGatekeeperVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherGatekeeperName, catalog.RancherChartRepo)
	require.NoError(g.T(), err)

	// Create project
	projectConfig := &management.Project{
		ClusterID: cluster.ID,
		Name:      gatekeeperProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(g.T(), err)
	require.Equal(g.T(), createdProject.Name, gatekeeperProjectName)
	g.project = createdProject

	g.gatekeeperChartInstallOptions = &charts.InstallOptions{
		Cluster:   cluster,
		Version:   latestGatekeeperVersion,
		ProjectID: createdProject.ID,
	}
}

func (g *GateKeeperTestSuite) TestGatekeeperChart() {
	subSession := g.session.NewSession()
	defer subSession.Cleanup()

	client, err := g.client.WithSession(subSession)
	require.NoError(g.T(), err)

	g.T().Log("Installing latest version of gatekeeper chart")
	err = charts.InstallRancherGatekeeperChart(client, g.gatekeeperChartInstallOptions)
	require.NoError(g.T(), err)

	g.T().Log("Waiting for gatekeeper chart deployments to have expected number of available replicas")
	err = extencharts.WatchAndWaitDeployments(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(g.T(), err)

	g.T().Log("Waiting for gatekeeper chart DaemonSets to have expected number of available nodes")
	err = extencharts.WatchAndWaitDaemonSets(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(g.T(), err)

	g.T().Log("Applying constraint")
	readYamlFile, err := os.ReadFile("./resources/opa-k8srequiredlabels.yaml")
	require.NoError(g.T(), err)
	yamlInput := &management.ImportClusterYamlInput{
		DefaultNamespace: charts.RancherGatekeeperNamespace,
		YAML:             string(readYamlFile),
	}

	// get the cluster
	cluster, err := client.Management.Cluster.ByID(g.project.ClusterID)
	require.NoError(g.T(), err)
	// Use ActionImportYaml to the apply the constraint yaml file
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlInput)
	require.NoError(g.T(), err)

	g.T().Log("Create a namespace that doesn't have the proper label and assert that creation fails with the expected error")
	_, err = namespaces.CreateNamespace(client, RancherDisallowedNamespace, "{}", map[string]string{}, map[string]string{}, g.project)
	assert.ErrorContains(g.T(), err, "Bad response statusCode [403]. Status [403 Forbidden].")
	g.T().Log("Waiting for gatekeeper audit to finish")
	err = getAuditTimestamp(client, g.project)
	require.NoError(g.T(), err)

	steveClient, err := client.Steve.ProxyDownstream(g.project.ClusterID)
	require.NoError(g.T(), err)

	// now that audit has run, get the list of constraints again
	constraintList, err := steveClient.SteveType(ConstraintResourceSteveType).List(nil)
	require.NoError(g.T(), err)

	// parse list of constraints
	constraintsStatusType := &ConstraintStatus{}
	constraintStatus := constraintList.Data[0].Status
	err = v1.ConvertToK8sType(constraintStatus, constraintsStatusType)
	require.NoError(g.T(), err)

	g.T().Log("getting list of all namespaces")
	namespacesList, err := steveClient.SteveType(namespaces.NamespaceSteveType).List(nil)
	require.NoError(g.T(), err)

	g.T().Log("getting list of namespaces with violations...")
	totalViolations := constraintsStatusType.TotalViolations
	// get the number of namespaces
	totalNamespaces := len(namespacesList.Data)

	g.T().Log("Asserting that all namespaces violate the constraint")
	assert.EqualValues(g.T(), totalNamespaces, totalViolations)
}

func (g *GateKeeperTestSuite) TestUpgradeGatekeeperChart() {
	subSession := g.session.NewSession()
	defer subSession.Cleanup()

	client, err := g.client.WithSession(subSession)
	require.NoError(g.T(), err)

	// Change gatekeeper install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.RancherGatekeeperName, catalog.RancherChartRepo)
	require.NoError(g.T(), err)

	if len(versionsList) < 2 {
		g.T().Skip("Skipping the upgrade case, only one version of gatekeeper is available")
	}
	assert.GreaterOrEqualf(g.T(), len(versionsList), 2, "There should be at least 2 versions of the gatekeeper chart")
	versionLatest := versionsList[0]
	g.T().Log(versionLatest)
	versionBeforeLatest := versionsList[1]
	g.T().Log(versionBeforeLatest)
	g.gatekeeperChartInstallOptions.Version = versionBeforeLatest

	g.T().Log("Checking if the gatekeeper chart is installed with one of the previous versions")
	initialGatekeeperChart, err := extencharts.GetChartStatus(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, charts.RancherGatekeeperName)
	require.NoError(g.T(), err)

	if initialGatekeeperChart.IsAlreadyInstalled && initialGatekeeperChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		g.T().Skip("Skipping the upgrade case, gatekeeper chart is already installed with the latest version")
	}

	if !initialGatekeeperChart.IsAlreadyInstalled {
		g.T().Log("Installing gatekeeper chart with the version before the latest version")
		err = charts.InstallRancherGatekeeperChart(client, g.gatekeeperChartInstallOptions)
		require.NoError(g.T(), err)

		g.T().Log("Waiting gatekeeper chart deployments to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
		require.NoError(g.T(), err)

		g.T().Log("Waiting gatekeeper chart DaemonSets to have expected number of available nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
		require.NoError(g.T(), err)
	}

	gatekeeperChartPreUpgrade, err := extencharts.GetChartStatus(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, charts.RancherGatekeeperName)
	require.NoError(g.T(), err)

	// Validate current version of rancher-gatekeeper is one of the versions before latest
	chartVersionPreUpgrade := gatekeeperChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	require.Contains(g.T(), versionsList[1:], chartVersionPreUpgrade)

	g.gatekeeperChartInstallOptions.Version, err = client.Catalog.GetLatestChartVersion(charts.RancherGatekeeperName, catalog.RancherChartRepo)
	require.NoError(g.T(), err)

	g.T().Log("Upgrading gatekeeper chart to the latest version")
	err = charts.UpgradeRancherGatekeeperChart(client, g.gatekeeperChartInstallOptions)
	require.NoError(g.T(), err)

	g.T().Log("Waiting for gatekeeper chart deployments to have expected number of available replicas after upgrade")
	err = extencharts.WatchAndWaitDeployments(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(g.T(), err)

	g.T().Log("Waiting gatekeeper chart DaemonSets to have expected number of available nodes after upgrade")
	err = extencharts.WatchAndWaitDaemonSets(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(g.T(), err)

	gatekeeperChartPostUpgrade, err := extencharts.GetChartStatus(client, g.project.ClusterID, charts.RancherGatekeeperNamespace, charts.RancherGatekeeperName)
	require.NoError(g.T(), err)

	g.T().Log("Comparing installed and desired gatekeeper versions")
	chartVersionPostUpgrade := gatekeeperChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	require.Equal(g.T(), g.gatekeeperChartInstallOptions.Version, chartVersionPostUpgrade)
}

func TestGateKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(GateKeeperTestSuite))
}
