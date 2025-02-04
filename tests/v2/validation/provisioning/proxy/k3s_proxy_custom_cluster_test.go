//go:build validation

package proxy

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/validation/pipeline/rancherha/corralha"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProxyK3SCustomClusterTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	corralPackage      *corral.Packages
	clustersConfig     *provisioninginput.Config
	EnvVar             rkev1.EnvVar
	corralImage        string
	corralAutoCleanup  bool
}

func (k *ProxyK3SCustomClusterTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *ProxyK3SCustomClusterTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	corralRancherHA := new(corralha.CorralRancherHA)
	config.LoadConfig(corralha.CorralRancherHAConfigConfigurationFileKey, corralRancherHA)

	k.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, k.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client

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
	require.NoError(k.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(k.T(), err)

	k.standardUserClient = standardUserClient

	k.clustersConfig.K3SKubernetesVersions, err = kubernetesversions.Default(
		k.client, clusters.K3SClusterType.String(), k.clustersConfig.K3SKubernetesVersions)
	require.NoError(k.T(), err)

	listOfCorrals, err := corral.ListCorral()
	require.NoError(k.T(), err)

	corralConfig := corral.Configurations()
	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(k.T(), err)

	k.corralPackage = corral.PackagesConfig()
	k.corralImage = k.corralPackage.CorralPackageImages[corralPackageAirgapCustomClusterName]
	k.corralAutoCleanup = k.corralPackage.HasCleanup

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

		err = corral.SetCorralSSHKeys(corralRancherHA.Name)
		require.NoError(k.T(), err)

		err = corral.SetCorralBastion(corralRancherHA.Name)
		require.NoError(k.T(), err)
	} else {
		k.T().Logf("Using AgentEnvVars from config: %v", k.clustersConfig.AgentEnvVars)
	}

}

func (k *ProxyK3SCustomClusterTestSuite) TestProxyK3SCustomClusterProvisioning() {
	k.clustersConfig.MachinePools = []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}

	tests := []struct {
		name    string
		client  *rancher.Client
		runFlag bool
	}{
		{provisioninginput.StandardClientName.String() + "-" + permutations.K3SAirgapCluster + "-", k.standardUserClient, k.standardUserClient.Flags.GetValue(environmentflag.Short)},
	}
	for _, tt := range tests {
		provisioningConfig := *k.clustersConfig
		permutations.RunTestPermutations(&k.Suite, tt.name, tt.client, &provisioningConfig, permutations.K3SAirgapCluster, nil, k.corralPackage)
	}
}

func TestProxyK3SCustomClusterTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/tfp-automation for updated tests")
	suite.Run(t, new(ProxyK3SCustomClusterTestSuite))
}
