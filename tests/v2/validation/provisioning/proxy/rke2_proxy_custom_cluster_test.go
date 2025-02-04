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

type ProxyRKE2CustomClusterTestSuite struct {
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

func (r *ProxyRKE2CustomClusterTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ProxyRKE2CustomClusterTestSuite) SetupSuite() {
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
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient

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
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)

		r.EnvVar.Name = "HTTPS_PROXY"
		r.EnvVar.Value = bastionIP + ":3219"
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)

		r.EnvVar.Name = "NO_PROXY"
		r.EnvVar.Value = "localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,cattle-system.svc"
		r.clustersConfig.AgentEnvVars = append(r.clustersConfig.AgentEnvVars, r.EnvVar)

		err = corral.SetCorralSSHKeys(corralRancherHA.Name)
		require.NoError(r.T(), err)

		err = corral.SetCorralBastion(corralRancherHA.Name)
		require.NoError(r.T(), err)
	} else {
		r.T().Logf("Using AgentEnvVars from config: %v", r.clustersConfig.AgentEnvVars)
	}

}

func (r *ProxyRKE2CustomClusterTestSuite) TestProxyRKE2CustomClusterProvisioning() {
	r.clustersConfig.MachinePools = []provisioninginput.MachinePools{provisioninginput.AllRolesMachinePool}

	tests := []struct {
		name    string
		client  *rancher.Client
		runFlag bool
	}{
		{provisioninginput.StandardClientName.String() + "-" + permutations.RKE2AirgapCluster + "-", r.standardUserClient, r.standardUserClient.Flags.GetValue(environmentflag.Short)},
	}
	for _, tt := range tests {
		provisioningConfig := *r.clustersConfig
		permutations.RunTestPermutations(&r.Suite, tt.name, tt.client, &provisioningConfig, permutations.RKE2AirgapCluster, nil, r.corralPackage)
	}
}

func TestProxyRKE2CustomClusterTestSuite(t *testing.T) {
	t.Skip("This test has been deprecated; check https://github.com/rancher/tfp-automation for updated tests")
	suite.Run(t, new(ProxyRKE2CustomClusterTestSuite))
}
