//go:build validation

package airgap

import (
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	provisioning "github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/registries"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AirGapRKE2CustomClusterTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	corralPackage      *corral.Packages
	clustersConfig     *provisioninginput.Config
	registryFQDN       string
}

func (a *AirGapRKE2CustomClusterTestSuite) TearDownSuite() {
	a.session.Cleanup()
}

func (a *AirGapRKE2CustomClusterTestSuite) SetupSuite() {
	testSession := session.NewSession()
	a.session = testSession

	a.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, a.clustersConfig)

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	registriesConfig := new(registries.Registries)
	config.LoadConfig(registries.RegistriesConfigKey, registriesConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(a.T(), err)

	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	enabled := true

	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(a.T(), err)

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(a.T(), err)

	a.client = standardUserClient

	listOfCorrals, err := corral.ListCorral()
	require.NoError(a.T(), err)

	corralConfig := corral.Configurations()
	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(a.T(), err)

	a.corralPackage = corral.PackagesConfig()

	_, corralExist := listOfCorrals[corralRancherHA.Name]
	if corralExist {
		bastionIP, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryIP)
		require.NoError(a.T(), err)

		err = corral.UpdateCorralConfig(corralBastionIP, bastionIP)
		require.NoError(a.T(), err)

		registryFQDN, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryFQDN)
		require.NoError(a.T(), err)
		logrus.Infof("registry fqdn is %s", registryFQDN)

		err = corral.SetCorralSSHKeys(corralRancherHA.Name)
		require.NoError(a.T(), err)

		a.registryFQDN = registryFQDN
	} else {
		a.registryFQDN = registriesConfig.ExistingNoAuthRegistryURL
	}
}

func (a *AirGapRKE2CustomClusterTestSuite) TestProvisioningAirGapRKE2CustomCluster() {
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	nodeRolesShared := []provisioninginput.MachinePools{provisioninginput.EtcdControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}

	tests := []struct {
		name        string
		client      *rancher.Client
		machinePool []provisioninginput.MachinePools
	}{
		{"1 Node all Roles " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesAll},
		{"2 nodes - etcd|cp roles per 1 node " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesShared},
		{"3 nodes - 1 role per node " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesDedicated},
	}
	for _, tt := range tests {
		a.clustersConfig.MachinePools = tt.machinePool

		if a.clustersConfig.RKE2KubernetesVersions == nil {
			rke2Versions, err := kubernetesversions.ListRKE2AllVersions(a.client)
			require.NoError(a.T(), err)

			a.clustersConfig.RKE2KubernetesVersions = rke2Versions
		}

		permutations.RunTestPermutations(&a.Suite, tt.name, tt.client, a.clustersConfig, permutations.RKE2AirgapCluster, nil, a.corralPackage)
	}
}

func (a *AirGapRKE2CustomClusterTestSuite) TestProvisioningAirGapUpgradeRKE2CustomCluster() {
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	nodeRolesShared := []provisioninginput.MachinePools{provisioninginput.EtcdControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	nodeRolesDedicated := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}

	tests := []struct {
		name        string
		client      *rancher.Client
		machinePool []provisioninginput.MachinePools
	}{
		{"Upgrading 1 node all Roles from " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesAll},
		{"Upgrading 2 nodes - etcd|cp roles per 1 node from " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesShared},
		{"Upgrading 3 nodes - 1 role per node from " + provisioninginput.StandardClientName.String(), a.standardUserClient, nodeRolesDedicated},
	}

	for _, tt := range tests {
		a.clustersConfig.MachinePools = tt.machinePool

		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(a.client)
		require.NoError(a.T(), err)

		require.Equal(a.T(), len(a.clustersConfig.CNIs), 1)

		if a.clustersConfig.RKE2KubernetesVersions != nil {
			rke2Versions = a.clustersConfig.RKE2KubernetesVersions
		}

		numOfRKE2Versions := len(rke2Versions)

		testConfig := clusters.ConvertConfigToClusterConfig(a.clustersConfig)
		testConfig.KubernetesVersion = rke2Versions[numOfRKE2Versions-2]
		testConfig.CNI = a.clustersConfig.CNIs[0]

		versionToUpgrade := rke2Versions[numOfRKE2Versions-1]
		tt.name += testConfig.KubernetesVersion + " to " + versionToUpgrade

		a.Run(tt.name, func() {
			clusterObject, err := provisioning.CreateProvisioningAirgapCustomCluster(a.client, testConfig, a.corralPackage)
			require.NoError(a.T(), err)

			reports.TimeoutClusterReport(clusterObject, err)
			require.NoError(a.T(), err)

			provisioning.VerifyCluster(a.T(), a.client, testConfig, clusterObject)

			updatedClusterObject := new(apisV1.Cluster)
			err = steveV1.ConvertToK8sType(clusterObject, &updatedClusterObject)
			require.NoError(a.T(), err)

			updatedClusterObject.Spec.KubernetesVersion = versionToUpgrade
			testConfig.KubernetesVersion = versionToUpgrade

			a.client, err = a.client.ReLogin()
			require.NoError(a.T(), err)

			upgradedCluster, err := extensionscluster.UpdateK3SRKE2Cluster(a.client, clusterObject, updatedClusterObject)
			require.NoError(a.T(), err)

			provisioning.VerifyCluster(a.T(), a.client, testConfig, upgradedCluster)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAirGapCustomClusterRKE2ProvisioningTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/tfp-automation for updated tests")
	suite.Run(t, new(AirGapRKE2CustomClusterTestSuite))
}
