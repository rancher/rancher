package charts

import (
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	// Project that charts are installed in
	deplymentNamespace    = "default"
	vsphereCpiProjectName = "system"
	CpiConfigFileKey      = "cpi"
	CsiConfigFileKey      = "csi"
)

// chartInstallOptions is a private struct that has cpi and csi charts install options
type vsphereChartInstallOptions struct {
	cpi *charts.InstallOptions
	csi *charts.InstallOptions
}

type CpiTestSuite struct {
	suite.Suite
	client                     *rancher.Client
	session                    *session.Session
	project                    *management.Project
	vsphereChartInstallOptions *vsphereChartInstallOptions
	cpiConfig                  *charts.CpiConfig
	csiConfig                  *charts.CsiConfig
}

func (c *CpiTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CpiTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	cpiConfig := new(charts.CpiConfig)
	csiConfig := new(charts.CsiConfig)

	config.LoadConfig(CpiConfigFileKey, cpiConfig)
	config.LoadConfig(CsiConfigFileKey, csiConfig)

	c.cpiConfig = cpiConfig
	c.csiConfig = csiConfig

	require.NotEmptyf(c.T(), cpiConfig.Datacenters, cpiConfig.Host, cpiConfig.Username, cpiConfig.Password, "CPI Config is not set properly")
	require.NotEmptyf(c.T(), csiConfig.Datacenters, csiConfig.Host, csiConfig.Username, csiConfig.Password, csiConfig.DataStoreURL, "CSI Config is not set properly")

	c.T().Log("Getting CPI Config")

	// Get clusterName from config yaml
	require.NotEmptyf(c.T(), client.RancherConfig.ClusterName, "Cluster name to install is not set")

	// Get clusterID with clusterName
	clusterID, err := clusters.GetClusterIDByName(client, client.RancherConfig.ClusterName)
	require.NoError(c.T(), err)

	// Get latest versions of cpi & csi charts
	latestCpiVersion, err := client.Catalog.GetLatestChartVersion(charts.VsphereCpiName)
	require.NoError(c.T(), err)
	latestCsiVersion, err := client.Catalog.GetLatestChartVersion(charts.VsphereCsiName)
	require.NoError(c.T(), err)

	// Create project
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      vsphereCpiProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(c.T(), err)
	require.Equal(c.T(), createdProject.Name, vsphereCpiProjectName)
	c.project = createdProject

	c.vsphereChartInstallOptions = &vsphereChartInstallOptions{
		cpi: &charts.InstallOptions{
			ClusterName: client.RancherConfig.ClusterName,
			ClusterID:   clusterID,
			Version:     latestCpiVersion,
			ProjectID:   createdProject.ID,
		},
		csi: &charts.InstallOptions{
			ClusterName: client.RancherConfig.ClusterName,
			ClusterID:   clusterID,
			Version:     latestCsiVersion,
			ProjectID:   createdProject.ID,
		},
	}
}

func (c *CpiTestSuite) TestCpiChart() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	client, err := c.client.WithSession(subSession)
	require.NoError(c.T(), err)

	c.T().Log("Checking if the cpi chart is installed")
	cpiChart, err := charts.GetChartStatus(client, c.project.ClusterID, charts.VsphereCpiNamespace, charts.VsphereCpiName)
	require.NoError(c.T(), err)

	if !cpiChart.IsAlreadyInstalled {
		c.T().Log("Installing cpi chart")
		err = charts.InstallVsphereCpiChart(client, c.vsphereChartInstallOptions.cpi, c.cpiConfig)
		require.NoError(c.T(), err)

		c.T().Log("Waiting cpi chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.VsphereCpiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting cpi chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, c.project.ClusterID, charts.VsphereCpiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting cpi chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, c.project.ClusterID, charts.VsphereCpiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)
	}

	c.T().Log("Checking if the csi chart is installed")
	csiChart, err := charts.GetChartStatus(client, c.project.ClusterID, charts.VsphereCsiNamespace, charts.VsphereCsiName)
	require.NoError(c.T(), err)

	if !csiChart.IsAlreadyInstalled {
		c.T().Log("Installing csi chart with the latest version")
		err = charts.InstallVsphereCsiChart(client, c.vsphereChartInstallOptions.csi, c.csiConfig)
		require.NoError(c.T(), err)

		c.T().Log("Waiting csi chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, charts.VsphereCsiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting csi chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, c.project.ClusterID, charts.VsphereCsiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

		c.T().Log("Waiting csi chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, c.project.ClusterID, charts.VsphereCsiNamespace, metav1.ListOptions{})
		require.NoError(c.T(), err)

	}

	c.T().Log("Creating a deployment with mounted volume")
	pvcYamlFile, err := os.ReadFile("./resources/persistent-volume-claim.yaml")
	require.NoError(c.T(), err)
	pvcYamlInput := &management.ImportClusterYamlInput{
		DefaultNamespace: deplymentNamespace,
		YAML:             string(pvcYamlFile),
	}
	pvccluster, err := client.Management.Cluster.ByID(c.project.ClusterID)
	require.NoError(c.T(), err)
	_, err = client.Management.Cluster.ActionImportYaml(pvccluster, pvcYamlInput)
	require.NoError(c.T(), err)

	c.T().Log("Waiting example app deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, deplymentNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

	c.T().Log("Creating a deployment with mounted volume")
	readYamlFile, err := os.ReadFile("./resources/deployment-for-volume.yaml")
	require.NoError(c.T(), err)
	yamlInput := &management.ImportClusterYamlInput{
		DefaultNamespace: deplymentNamespace,
		YAML:             string(readYamlFile),
	}
	cluster, err := client.Management.Cluster.ByID(c.project.ClusterID)
	require.NoError(c.T(), err)
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlInput)
	require.NoError(c.T(), err)

	c.T().Log("Waiting example app deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, c.project.ClusterID, deplymentNamespace, metav1.ListOptions{})
	require.NoError(c.T(), err)

}

func TestCpiTestSuite(t *testing.T) {
	suite.Run(t, new(CpiTestSuite))
}
