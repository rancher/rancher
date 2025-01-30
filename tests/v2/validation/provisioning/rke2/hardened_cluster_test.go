//go:build (validation || sanity) && !infra.any && !infra.aks && !infra.eks && !infra.rke2k3s && !infra.gke && !infra.rke1 && !cluster.any && !cluster.custom && !cluster.nodedriver && !extended && !stress

package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	cis "github.com/rancher/rancher/tests/v2/validation/provisioning/resources/cisbenchmark"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HardenedRKE2ClusterProvisioningTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	standardUserClient  *rancher.Client
	provisioningConfig  *provisioninginput.Config
	project             *management.Project
	chartInstallOptions *charts.InstallOptions
	chartFeatureOptions *charts.RancherMonitoringOpts
}

func (c *HardenedRKE2ClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *HardenedRKE2ClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	c.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, c.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	if c.provisioningConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.Default(c.client, extensionscluster.RKE2ClusterType.String(), nil)
		require.NoError(c.T(), err)

		c.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	} else if c.provisioningConfig.RKE2KubernetesVersions[0] == "all" {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(c.client)
		require.NoError(c.T(), err)

		c.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	}

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(c.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(c.T(), err)

	c.standardUserClient = standardUserClient
}

func (c *HardenedRKE2ClusterProvisioningTestSuite) TestProvisioningRKE2HardenedCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}

	tests := []struct {
		name            string
		client          *rancher.Client
		machinePools    []provisioninginput.MachinePools
		scanProfileName string
	}{
		{"RKE2 CIS 1.8 Profile Hardened " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicated, "rke2-cis-1.8-profile-hardened"},
		{"RKE2 CIS 1.8 Profile Permissive " + provisioninginput.StandardClientName.String(), c.standardUserClient, nodeRolesDedicated, "rke2-cis-1.8-profile-permissive"},
	}
	for _, tt := range tests {
		c.Run(tt.name, func() {
			provisioningConfig := *c.provisioningConfig
			provisioningConfig.MachinePools = tt.machinePools
			provisioningConfig.Hardened = true

			nodeProviders := provisioningConfig.NodeProviders[0]
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)

			testConfig := clusters.ConvertConfigToClusterConfig(&provisioningConfig)
			testConfig.KubernetesVersion = c.provisioningConfig.RKE2KubernetesVersions[0]

			clusterObject, err := provisioning.CreateProvisioningCustomCluster(tt.client, &externalNodeProvider, testConfig)
			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(c.T(), err)

			provisioning.VerifyCluster(c.T(), tt.client, testConfig, clusterObject)

			cluster, err := extensionscluster.NewClusterMeta(tt.client, clusterObject.Name)
			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(c.T(), err)

			latestCISBenchmarkVersion, err := tt.client.Catalog.GetLatestChartVersion(charts.CISBenchmarkName, catalog.RancherChartRepo)
			require.NoError(c.T(), err)

			project, err := projects.GetProjectByName(tt.client, cluster.ID, cis.System)
			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(c.T(), err)

			c.project = project
			require.NotEmpty(c.T(), c.project)

			c.chartInstallOptions = &charts.InstallOptions{
				Cluster:   cluster,
				Version:   latestCISBenchmarkVersion,
				ProjectID: c.project.ID,
			}

			cis.SetupCISBenchmarkChart(tt.client, c.project.ClusterID, c.chartInstallOptions, charts.CISBenchmarkNamespace)
			cis.RunCISScan(tt.client, c.project.ClusterID, tt.scanProfileName)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHardenedRKE2ClusterProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(HardenedRKE2ClusterProvisioningTestSuite))
}
