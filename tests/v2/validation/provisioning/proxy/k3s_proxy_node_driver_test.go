package proxy

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/permutations"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProxyK3SProvisioningTestSuite struct {
	suite.Suite
	client         *rancher.Client
	session        *session.Session
	clustersConfig *provisioninginput.Config
	EnvVar         rkev1.EnvVar
}

func (k *ProxyK3SProvisioningTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *ProxyK3SProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	k.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, k.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client

	k.clustersConfig.K3SKubernetesVersions, err = kubernetesversions.Default(
		k.client, clusters.K3SClusterType.String(), k.clustersConfig.K3SKubernetesVersions)
	require.NoError(k.T(), err)

	listOfCorrals, err := corral.ListCorral()
	require.NoError(k.T(), err)

	_, corralExist := listOfCorrals[corralRancherHA.Name]
	if corralExist {
		bastionIP, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryPrivateIP)
		require.NoError(k.T(), err)

		k.EnvVar.Name = "HTTP_PROXY"
		k.EnvVar.Value = bastionIP + ":3219"
		k.clustersConfig.AgentEnvVars = append(k.clustersConfig.AgentEnvVars, k.EnvVar)

		k.EnvVar.Name = "HTTPS_PROXY"
		k.EnvVar.Value = bastionIP + ":3219"
		k.clustersConfig.AgentEnvVars = append(k.clustersConfig.AgentEnvVars, k.EnvVar)

		k.EnvVar.Name = "NO_PROXY"
		k.EnvVar.Value = "localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,cattle-system.svc"
		k.clustersConfig.AgentEnvVars = append(k.clustersConfig.AgentEnvVars, k.EnvVar)
	} else {
		k.T().Logf("Using AgentEnvVars from config: %v", k.clustersConfig.AgentEnvVars)
	}
}

func (k *ProxyK3SProvisioningTestSuite) TestProxyK3SClusterProvisioning() {
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		runFlag      bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRolesAll, k.client, k.client.Flags.GetValue(environmentflag.Short)},
	}

	for _, tt := range tests {
		if !tt.runFlag {
			k.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *k.clustersConfig
		provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
	}
}

func TestProxyK3SProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyK3SProvisioningTestSuite))
}
