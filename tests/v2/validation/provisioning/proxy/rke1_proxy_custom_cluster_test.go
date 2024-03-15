package proxy

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/permutations"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProxyRKE1CustomClusterTestSuite struct {
	suite.Suite
	client            *rancher.Client
	session           *session.Session
	corralPackage     *corral.Packages
	clustersConfig    *provisioninginput.Config
	EnvVar            management.EnvVar
	corralImage       string
	corralAutoCleanup bool
}

func (r *ProxyRKE1CustomClusterTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ProxyRKE1CustomClusterTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	listOfCorrals, err := corral.ListCorral()
	require.NoError(r.T(), err)

	corralConfig := corral.Configurations()
	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(r.T(), err)

	r.corralPackage = corral.PackagesConfig()
	r.corralImage = r.corralPackage.CorralPackageImages[corralPackageAirgapCustomClusterName]
	r.corralAutoCleanup = r.corralPackage.HasCleanup

	_, corralExist := listOfCorrals[corralRancherHA.Name]
	if corralExist {
		bastionIP, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryPrivateIP)
		require.NoError(r.T(), err)

		r.EnvVar.Name = "HTTP_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.clustersConfig.AgentEnvVarsRKE1 = append(r.clustersConfig.AgentEnvVarsRKE1, r.EnvVar)

		r.EnvVar.Name = "HTTPS_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.clustersConfig.AgentEnvVarsRKE1 = append(r.clustersConfig.AgentEnvVarsRKE1, r.EnvVar)

		r.EnvVar.Name = "NO_PROXY"
		r.EnvVar.Value = "localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,cattle-system.svc"
		r.clustersConfig.AgentEnvVarsRKE1 = append(r.clustersConfig.AgentEnvVarsRKE1, r.EnvVar)

		err = corral.SetCorralSSHKeys(corralRancherHA.Name)
		require.NoError(r.T(), err)

		err = corral.SetCorralBastion(corralRancherHA.Name)
		require.NoError(r.T(), err)
	} else {
		r.T().Logf("Using AgentEnvVarsRKE1 from config: %v", r.clustersConfig.AgentEnvVarsRKE1)
	}
}

func (r *ProxyRKE1CustomClusterTestSuite) TestProxyRKE1CustomCluster() {
	r.clustersConfig.NodePools = []provisioninginput.NodePools{provisioninginput.AllRolesNodePool}

	tests := []struct {
		name    string
		client  *rancher.Client
		runFlag bool
	}{
		{provisioninginput.AdminClientName.String() + "-" + permutations.RKE1AirgapCluster + "-", r.client, r.client.Flags.GetValue(environmentflag.Short)},
	}
	for _, tt := range tests {
		provisioningConfig := *r.clustersConfig
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE1AirgapCluster, nil, r.corralPackage)
	}

}

func TestProxyRKE1CustomClusterTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyRKE1CustomClusterTestSuite))
}
