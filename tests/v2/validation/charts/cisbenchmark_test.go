package charts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	Error            = "error"
	Fail             = "fail"
	scanPass         = "pass"
	scanFail         = "fail"
	CisConfigFileKey = "cis"
)

type CisConfig struct {
	ProfileName  string `json:"profileName" yaml:"profileName"`
	ChartVersion string `json:"chartVersion" yaml:"chartVersion"`
}

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

	cisConfig := new(CisConfig)
	config.LoadConfig(CisConfigFileKey, cisConfig)
	c.cisProfileName = cisConfig.ProfileName
	require.NotEmptyf(c.T(), c.cisProfileName, "CIS profile name required")

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(c.T(), clusterName, "Cluster name to install is not set")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)

	if cisConfig.ChartVersion == "" {
		cisConfig.ChartVersion, err = client.Catalog.GetLatestChartVersion(charts.RancherCisBenchmarkName, catalog.RancherChartRepo)
		require.NoError(c.T(), err)
	}

	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      charts.CisbenchmarkProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(c.T(), err)
	require.Equal(c.T(), createdProject.Name, charts.CisbenchmarkProjectName)
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

	c.T().Log("Installing latest version of cis benchmark chart")
	err = charts.InstallRancherCisBenchmarkChart(client, c.chartInstallOptions)
	require.NoError(c.T(), err)

	c.T().Log("Waiting for cisbenchmark chart deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

	scanName := namegen.AppendRandomString("scan")

	c.T().Log("Running a scan")

	// Run the scan
	err = runScan(client, scanName, c.cisProfileName, c.project.ClusterID)
	require.NoError(c.T(), err)

}

func (c *CisBenchmarkTestSuite) TestUpgradeCisbenchmarkChart() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	// Change cisbenchmark install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.RancherCisBenchmarkName, catalog.RancherChartRepo)
	require.NoError(c.T(), err)

	if len(versionsList) < 2 {
		c.T().Skip("Skipping the upgrade case, only one version of cis-benchmark is available")
	}
	assert.GreaterOrEqualf(c.T(), len(versionsList), 2, "There should be at least 2 versions of the cis-benchmark chart")
	versionLatest := versionsList[0]
	c.T().Log(versionLatest)

	versionBeforeLatest := versionsList[1]
	c.T().Log(versionBeforeLatest)
	c.chartInstallOptions.Version = versionBeforeLatest

	c.T().Log("Checking if the cis-benchmark chart is installed with one of the previous versions")
	initialCisbenchmarkChart, err := charts.GetChartStatus(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, charts.RancherCisBenchmarkName)
	require.NoError(c.T(), err)

	if initialCisbenchmarkChart.IsAlreadyInstalled && initialCisbenchmarkChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		c.T().Skip("Skipping the upgrade case, cis-benchmark chart is already installed with the latest version")
	}

	if !initialCisbenchmarkChart.IsAlreadyInstalled {
		c.T().Log("Installing cis-benchmark chart with the version before the latest version")
		err = charts.InstallRancherCisBenchmarkChart(client, c.chartInstallOptions)
		require.NoError(c.T(), err)

		c.T().Log("Waiting cis-benchmark chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting cisbenchmark chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)
	}

	cisbenchmarkChartPreUpgrade, err := charts.GetChartStatus(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, charts.RancherCisBenchmarkName)
	require.NoError(c.T(), err)

	// Validate current version of rancher-cis-benchmark is one of the versions before latest
	chartVersionPreUpgrade := cisbenchmarkChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	require.Contains(c.T(), versionsList[1:], chartVersionPreUpgrade)

	c.chartInstallOptions.Version, err = client.Catalog.GetLatestChartVersion(charts.RancherCisBenchmarkName, catalog.RancherChartRepo)
	require.NoError(c.T(), err)
	scanName := namegen.AppendRandomString("scan")

	c.T().Log("Running a scan")

	// Run the scan
	err = runScan(client, scanName, c.cisProfileName, c.project.ClusterID)
	require.NoError(c.T(), err)

	require.NoError(c.T(), err)

	c.T().Log("Upgrading cis-benchmark chart to the latest version")
	err = charts.UpgradeRancherCisBenchmarkChart(client, c.chartInstallOptions)
	require.NoError(c.T(), err)

	c.T().Log("Waiting for cis-benchmark chart deployments to have expected number of available replicas after upgrade")
	err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

	c.T().Log("Waiting cis-benchmark chart DaemonSets to have expected number of available nodes after upgrade")
	err = charts.WatchAndWaitDaemonSets(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

	cisbenchmarkChartPostUpgrade, err := charts.GetChartStatus(client, c.project.ClusterID, charts.RancherCisBenchmarkNamespace, charts.RancherCisBenchmarkName)
	require.NoError(c.T(), err)

	c.T().Log("Comparing installed and desired cis-benchmark versions")
	chartVersionPostUpgrade := cisbenchmarkChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	require.Equal(c.T(), c.chartInstallOptions.Version, chartVersionPostUpgrade)

	scanName = namegen.AppendRandomString("scan")

	c.T().Log("Running a scan")

	// Run the scan
	err = runScan(client, scanName, c.cisProfileName, c.project.ClusterID)
	require.NoError(c.T(), err)
}

func getClusterScanTemplate() (string, error) {
	readYamlFile, err := os.ReadFile("./resources/cis-profiles/cluster-scan-template.yaml")
	return string(readYamlFile), err
}

func findScanIdx(data []v1.SteveAPIObject, scanName string) int {
	for idx, scanReport := range data {
		if scanReport.Name == scanName {
			return idx
		}
	}
	return -1
}

func checkScanReport(steveClient *v1.Client, scanName string) error {
	scanReportList, err := steveClient.SteveType(charts.ClusterScanReportType).List(nil)
	if err != nil {
		return err
	}

	scanReportIdx := findScanIdx(scanReportList.Data, scanName)

	for idx, scanReport := range scanReportList.Data {
		if strings.Contains(scanReport.Name, scanName) {
			scanReportIdx = idx
			break
		}
	}

	if scanReportIdx < 0 {
		return errors.New("scan report not found")
	}

	cisReportSpec := &charts.ClusterScanReportSpec{}
	scanReportSpec := scanReportList.Data[scanReportIdx].Spec
	err = v1.ConvertToK8sType(scanReportSpec, cisReportSpec)
	if err != nil {
		return err
	}

	reportData := &charts.CisReport{}
	err = json.Unmarshal([]byte(cisReportSpec.ReportJson), &reportData)
	if err != nil {
		return err
	}

	logrus.Infof("Out of total number of %d scans, %d scans passed, %d scans were skipped and %d scans failed", reportData.Total, reportData.Pass, reportData.Skip, reportData.Fail)
	for _, group := range reportData.Results {
		for _, check := range group.Checks {
			if check.State == Fail {
				logrus.Infof("check failed: id: %s state: %s description: %s ", check.Id, check.State, check.Description)
			}
		}
	}
	if reportData.Fail != 0 {
		return errors.New("cluster scan failed")
	}
	return nil
}

func runScan(client *rancher.Client, scanName, profileName, clusterID string) error {
	clusterScanTemplate, err := getClusterScanTemplate()
	if err != nil {
		return err
	}

	yamlInput := &management.ImportClusterYamlInput{
		DefaultNamespace: charts.RancherCisBenchmarkNamespace,
		YAML:             fmt.Sprintf(clusterScanTemplate, scanName, profileName),
	}

	// Get the cluster by ID using the client
	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	// Import YAML into the cluster
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlInput)
	if err != nil {
		return err
	}

	// Get a client for interacting with resources in the cluster
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	// Poll until the scan is completed
	err = wait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.FiveMinuteTimeout, func() (done bool, err error) {
		scanList, err := steveClient.SteveType(charts.ClusterScanResourceType).List(nil)
		if err != nil {
			return false, err
		}

		scanIdx := findScanIdx(scanList.Data, scanName)

		if scanIdx < 0 {
			return false, errors.New("scan not found")
		}

		clusterScanStatusType := &charts.ClusterScanStatus{}
		scanStatus := scanList.Data[scanIdx].Status
		err = v1.ConvertToK8sType(scanStatus, clusterScanStatusType)
		if err != nil {
			return false, err
		}

		if clusterScanStatusType.Display.State == Error {
			return false, errors.New(clusterScanStatusType.Display.Message)
		}
		if clusterScanStatusType.Display.State == scanPass ||
			clusterScanStatusType.Display.State == scanFail {
			logrus.Infof("cluster scan - %s", clusterScanStatusType.Display.State)
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	// Check the scan report
	err = checkScanReport(steveClient, scanName)
	if err != nil {
		return err
	}

	return nil
}

func TestCisBenchmarkTestSuite(t *testing.T) {
	suite.Run(t, new(CisBenchmarkTestSuite))
}
