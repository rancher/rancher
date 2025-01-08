package charts

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/validation/charts/resources"
	cis "github.com/rancher/rancher/tests/v2/validation/provisioning/resources/cisbenchmark"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CisBenchmarkTestSuite struct {
	suite.Suite
	cisProfileName      string
	client              *rancher.Client
	session             *session.Session
	project             *management.Project
	chartInstallOptions *charts.InstallOptions
}

func (c *CisBenchmarkTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CisBenchmarkTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	cisConfig := new(resources.CisConfig)
	config.LoadConfig(resources.CisConfigFileKey, cisConfig)
	c.cisProfileName = cisConfig.ProfileName
	require.NotEmptyf(c.T(), c.cisProfileName, "CIS profile name required")

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(c.T(), clusterName, "Cluster name to install is not set")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)

	if cisConfig.ChartVersion == "" {
		cisConfig.ChartVersion, err = client.Catalog.GetLatestChartVersion(charts.CISBenchmarkName, catalog.RancherChartRepo)
		require.NoError(c.T(), err)
	}

	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      resources.CISBenchmarkProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(c.T(), err)
	require.Equal(c.T(), createdProject.Name, resources.CISBenchmarkProjectName)
	c.project = createdProject

	c.chartInstallOptions = &charts.InstallOptions{
		Cluster: &clusters.ClusterMeta{
			ID:   clusterID,
			Name: clusterName,
		},
		Version:   cisConfig.ChartVersion,
		ProjectID: createdProject.ID,
	}

}

func (c *CisBenchmarkTestSuite) TestFreshInstallCisBenchmarkChart() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	c.T().Log("Checking if the cis-benchmark chart is already installed")
	initialCisbenchmarkChart, err := extencharts.GetChartStatus(client, c.project.ClusterID, charts.CISBenchmarkNamespace, charts.CISBenchmarkName)
	require.NoError(c.T(), err)

	if initialCisbenchmarkChart.IsAlreadyInstalled {
		c.T().Skip("Skipping fresh installtion. cis-benchmark chart is already installed.")
	}

	c.T().Log("Installing latest version of cis benchmark chart")
	err = charts.InstallCISBenchmarkChart(client, c.chartInstallOptions)
	require.NoError(c.T(), err)

	c.T().Log("Waiting for cisbenchmark chart deployments to have expected number of available replicas")
	err = extencharts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.CISBenchmarkNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

	c.T().Log("Running a scan")

	// Run the scan
	err = cis.RunCISScan(client, c.project.ClusterID, c.cisProfileName, true)
	require.NoError(c.T(), err)

}

func (c *CisBenchmarkTestSuite) TestDynamicUpgradeCisbenchmarkChart() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	// Change cisbenchmark install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.CISBenchmarkName, catalog.RancherChartRepo)
	require.NoError(c.T(), err)

	if len(versionsList) < 2 {
		c.T().Skip("Skipping the upgrade case, only one version of cis-benchmark is available")
	}

	c.T().Log("Checking if the cis-benchmark chart is installed with one of the previous versions")
	initialCisbenchmarkChart, err := extencharts.GetChartStatus(client, c.project.ClusterID, charts.CISBenchmarkNamespace, charts.CISBenchmarkName)
	require.NoError(c.T(), err)

	if !initialCisbenchmarkChart.IsAlreadyInstalled {
		c.T().Skip("cis-benchmark chart is not installed, skipping the upgrade case")
	}

	versionLatest := versionsList[0]
	c.T().Log(versionLatest)

	versionBeforeLatest := versionsList[1]
	c.T().Log(versionBeforeLatest)
	c.chartInstallOptions.Version = versionBeforeLatest

	if initialCisbenchmarkChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		c.T().Skip("Skipping the upgrade case, latest version of cis-benchmark chart is already installed")
	}

	c.T().Log("Running a scan")

	// Run the scan
	err = cis.RunCISScan(client, c.project.ClusterID, c.cisProfileName, true)
	require.NoError(c.T(), err)

	c.T().Log("Upgrading cis-benchmark chart to the latest version")
	c.chartInstallOptions.Version = versionLatest
	err = charts.UpgradeCISBenchmarkChart(client, c.chartInstallOptions)
	require.NoError(c.T(), err)

	c.T().Log("Verifying cis-benchmark chart installation")

	err = resources.VerifyCISChartInstallation(client, c.project.ClusterID, versionLatest, c.cisProfileName)
	require.NoError(c.T(), err)
}

func (c *CisBenchmarkTestSuite) TestUpgradeCisbenchmarkChart() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	// Change cisbenchmark install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.CISBenchmarkName, catalog.RancherChartRepo)
	require.NoError(c.T(), err)

	if len(versionsList) < 2 {
		c.T().Skip("Skipping the upgrade case, only one version of cis-benchmark is available")
	}

	versionLatest := versionsList[0]
	c.T().Log(versionLatest)

	versionBeforeLatest := versionsList[1]
	c.T().Log(versionBeforeLatest)
	c.chartInstallOptions.Version = versionBeforeLatest

	c.T().Log("Checking if the cis-benchmark chart is installed with one of the previous versions")
	initialCisbenchmarkChart, err := extencharts.GetChartStatus(client, c.project.ClusterID, charts.CISBenchmarkNamespace, charts.CISBenchmarkName)
	require.NoError(c.T(), err)

	if initialCisbenchmarkChart.IsAlreadyInstalled && initialCisbenchmarkChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		c.T().Skip("Skipping the upgrade case, cis-benchmark chart is already installed with the latest version")
	}

	if !initialCisbenchmarkChart.IsAlreadyInstalled {
		c.T().Log("Installing cis-benchmark chart with the version before the latest version")
		err = charts.InstallCISBenchmarkChart(client, c.chartInstallOptions)
		require.NoError(c.T(), err)

		c.T().Log("Waiting cis-benchmark chart deployments to have expected number of available replicas")
		err = extencharts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.CISBenchmarkNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting cisbenchmark chart DaemonSets to have expected number of available nodes")
		err = extencharts.WatchAndWaitDaemonSets(client, c.project.ClusterID, charts.CISBenchmarkNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)
	}

	c.T().Log("Running a scan")

	// Run the scan
	err = cis.RunCISScan(client, c.project.ClusterID, c.cisProfileName, true)
	require.NoError(c.T(), err)

	c.T().Log("Upgrading cis-benchmark chart to the latest version")
	c.chartInstallOptions.Version = versionLatest
	err = charts.UpgradeCISBenchmarkChart(client, c.chartInstallOptions)
	require.NoError(c.T(), err)

	c.T().Log("Verifying cis-benchmark chart installation")

	err = resources.VerifyCISChartInstallation(client, c.project.ClusterID, versionLatest, c.cisProfileName)
	require.NoError(c.T(), err)
}

func TestCisBenchmarkTestSuite(t *testing.T) {
	suite.Run(t, new(CisBenchmarkTestSuite))
}
