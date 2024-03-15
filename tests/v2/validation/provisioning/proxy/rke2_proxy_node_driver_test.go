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

type ProxyRKE2ProvisioningTestSuite struct {
	suite.Suite
	client         *rancher.Client
	session        *session.Session
	clustersConfig *provisioninginput.Config
	EnvVar         rkev1.EnvVar
}

func (r *ProxyRKE2ProvisioningTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ProxyRKE2ProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clustersConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(
		r.client, clusters.RKE2ClusterType.String(), r.clustersConfig.RKE2KubernetesVersions)
	require.NoError(r.T(), err)

	listOfCorrals, err := corral.ListCorral()
	require.NoError(r.T(), err)

	_, corralExist := listOfCorrals[corralRancherHA.Name]
	if corralExist {
		bastionIP, err := corral.GetCorralEnvVar(corralRancherHA.Name, corralRegistryPrivateIP)
		require.NoError(r.T(), err)

		r.EnvVar.Name = "HTTP_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)

		r.EnvVar.Name = "HTTPS_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)

		r.EnvVar.Name = "NO_PROXY"
		r.EnvVar.Value = "localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,cattle-system.svc"
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)
	} else {
		r.T().Logf("Using AgentEnvVars from config: %v", r.clustersConfig.AgentEnvVars)
	}
}

func (r *ProxyRKE2ProvisioningTestSuite) TestProxyRKE2ClusterProvisioning() {
	nodeRolesAll := []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}
	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		runFlag      bool
	}{
		{"1 Node all roles " + provisioninginput.AdminClientName.String(), nodeRolesAll, r.client, r.client.Flags.GetValue(environmentflag.Short)},
	}
	for _, tt := range tests {
		if !tt.runFlag {
			r.T().Logf("SKIPPED")
			continue
		}
		provisioningConfig := *r.clustersConfig
		provisioningConfig.MachinePools = tt.machinePools
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func TestProxyRKE2ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyRKE2ProvisioningTestSuite))
}
