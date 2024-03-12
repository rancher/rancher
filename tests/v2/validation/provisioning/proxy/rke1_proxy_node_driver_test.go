package proxy

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/permutations"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProxyRKE1ProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
	EnvVar             management.EnvVar
}

func (r *ProxyRKE1ProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ProxyRKE1ProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.provisioningConfig.RKE1KubernetesVersions, err = kubernetesversions.Default(
		r.client, clusters.RKE1ClusterType.String(), r.provisioningConfig.RKE1KubernetesVersions)
	require.NoError(r.T(), err)

	listOfCorrals, err := corral.ListCorral()
	require.NoError(r.T(), err)

	_, corralExist := listOfCorrals[corralRancherHA.Name]
	if corralExist {
		bastionIP, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryPrivateIP)
		require.NoError(r.T(), err)

		r.EnvVar.Name = "HTTP_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.provisioningConfig.AgentEnvVarsRKE1 = append(r.provisioningConfig.AgentEnvVarsRKE1, r.EnvVar)

		r.EnvVar.Name = "HTTPS_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.provisioningConfig.AgentEnvVarsRKE1 = append(r.provisioningConfig.AgentEnvVarsRKE1, r.EnvVar)

		r.EnvVar.Name = "NO_PROXY"
		r.EnvVar.Value = "localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,cattle-system.svc"
		r.provisioningConfig.AgentEnvVarsRKE1 = append(r.provisioningConfig.AgentEnvVarsRKE1, r.EnvVar)
	} else {
		r.T().Logf("Using AgentEnvVarsRKE1 from config: %v", r.provisioningConfig.AgentEnvVarsRKE1)
	}
}

func (r *ProxyRKE1ProvisioningTestSuite) TestProxyRKE1ClusterProvisioning() {
	nodeRolesAll := []provisioninginput.NodePools{provisioninginput.AllRolesNodePool}

	tests := []struct {
		name      string
		nodePools []provisioninginput.NodePools
		client    *rancher.Client
		runFlag   bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRolesAll, r.client, r.client.Flags.GetValue(environmentflag.Short)},
	}
	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *r.provisioningConfig
		provisioningConfig.NodePools = tt.nodePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
	}
}

func TestProxyRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyRKE1ProvisioningTestSuite))
}
