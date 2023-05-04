package registries

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/registries"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/rke1"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	corralRancherName      = "rancherha"
	corralAuthDisabledName = "registryauthdisabled"
	corralAuthEnabledName  = "registryauthenabled"
)

type RegistryTestSuite struct {
	suite.Suite
	session                        *session.Session
	client                         *rancher.Client
	clusterAuthID                  string
	clusterNoAuthID                string
	clusterLocalID                 string
	clusterAuthRegistryHost        string
	clusterNoAuthRegistryHost      string
	localClusterGlobalRegistryHost string
	rancherUsesRegistry            bool
	advancedOptions                provisioning.AdvancedOptions
}

func (rt *RegistryTestSuite) TearDownSuite() {
	rt.session.Cleanup()
}

func (rt *RegistryTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rt.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rt.T(), err)
	rt.client = client

	corralConfig := corral.CorralConfigurations()
	registriesConfig := new(Registries)
	config.LoadConfig(RegistriesConfigKey, registriesConfig)

	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(rt.T(), err)
	configPackage := corral.CorralPackagesConfig()

	globalRegistryFqdn := ""
	registryDisabledFqdn := ""
	registryEnabledUsername := ""
	registryEnabledPassword := ""
	registryEnabledFqdn := ""

	useRegistries := client.Flags.GetValue(environmentflag.UseExistingRegistries)
	logrus.Infof("The value of useRegistries is %t", useRegistries)

	if !useRegistries {
		for _, name := range registriesConfig.RegistryConfigNames {
			path := configPackage.CorralPackageImages[name]
			logrus.Infof("PATH: %s", path)

			_, err = corral.CreateCorral(testSession, name, path, true, configPackage.HasCleanup)
			if err != nil {
				logrus.Errorf("error creating corral: %v", err)
			}
		}
		registryDisabledFqdn, err = corral.GetCorralEnvVar(corralAuthDisabledName, "registry_fqdn")
		require.NoError(rt.T(), err)
		logrus.Infof("RegistryNoAuth FQDN %s", registryDisabledFqdn)
		registryEnabledUsername, err = corral.GetCorralEnvVar(corralAuthEnabledName, "registry_username")
		require.NoError(rt.T(), err)
		logrus.Infof("RegistryAuth Username %s", registryEnabledUsername)
		registryEnabledPassword, err = corral.GetCorralEnvVar(corralAuthEnabledName, "registry_password")
		require.NoError(rt.T(), err)
		logrus.Infof("RegistryAuth Password %s", registryEnabledPassword)
		registryEnabledFqdn, err = corral.GetCorralEnvVar(corralAuthEnabledName, "registry_fqdn")
		require.NoError(rt.T(), err)
		logrus.Infof("RegistryAuth FQDN %s", registryEnabledFqdn)
	} else {
		logrus.Infof("Using Existing Registries because value of useRegistries is %t", useRegistries)
		registryDisabledFqdn = registriesConfig.ExistingNoAuthRegistryURL
		registryEnabledFqdn = registriesConfig.ExistingAuthRegistryInfo.URL
		registryEnabledUsername = registriesConfig.ExistingAuthRegistryInfo.Username
		registryEnabledPassword = registriesConfig.ExistingAuthRegistryInfo.Password
		logrus.Infof("RegistryNoAuth FQDN %s", registryDisabledFqdn)
		logrus.Infof("RegistryAuth Username %s", registryEnabledUsername)
		logrus.Infof("RegistryAuth Password %s", registryEnabledPassword)
		logrus.Infof("RegistryAuth FQDN %s", registryEnabledFqdn)
	}

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	kubernetesVersions := clustersConfig.RKE1KubernetesVersions
	cnis := clustersConfig.CNIs
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1
	providers := clustersConfig.Providers
	rt.advancedOptions = clustersConfig.AdvancedOptions

	rt.rancherUsesRegistry = false
	listOfCorrals, err := corral.ListCorral()
	require.NoError(rt.T(), err)
	_, corralExist := listOfCorrals[corralRancherName]
	if corralExist {
		globalRegistryFqdn, err = corral.GetCorralEnvVar(corralRancherName, "registry_fqdn")
		require.NoError(rt.T(), err)
		if globalRegistryFqdn != "<nil>" {
			rt.rancherUsesRegistry = true
			logrus.Infof("Rancher Global Registry FQDN %s", globalRegistryFqdn)
		}
		logrus.Infof("Rancher was built using corral: %t", corralExist)
		logrus.Infof("Is Rancher using a global registry: %t", rt.rancherUsesRegistry)
	} else {
		if useRegistries {
			globalRegistryFqdn = registriesConfig.ExistingNoAuthRegistryURL
			rt.rancherUsesRegistry = true
			logrus.Infof("Rancher was built using corral: %t", corralExist)
			logrus.Infof("Is Rancher using a global registry: %t", rt.rancherUsesRegistry)
			logrus.Infof("Rancher Global Registry FQDN %s", globalRegistryFqdn)
		} else {
			rt.rancherUsesRegistry = false
			logrus.Infof("Rancher was built using corral: %t", corralExist)
			logrus.Infof("Is Rancher using a global registry: %t", rt.rancherUsesRegistry)
		}
	}

	var privateRegistriesNoAuth []management.PrivateRegistry
	privateRegistry := management.PrivateRegistry{}
	privateRegistry.URL = registryDisabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = ""
	privateRegistry.User = ""
	privateRegistriesNoAuth = append(privateRegistriesNoAuth, privateRegistry)

	var privateRegistriesAuth []management.PrivateRegistry
	privateRegistry = management.PrivateRegistry{}
	privateRegistry.URL = registryEnabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = registryEnabledPassword
	privateRegistry.User = registryEnabledUsername
	privateRegistriesAuth = append(privateRegistriesAuth, privateRegistry)

	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := client.WithSession(subSession)
	require.NoError(rt.T(), err)

	provider := rke1.CreateProvider(providers[0])
	clusterNameNoAuth, err := rt.testProvisionRKE1Cluster(subClient, provider, nodesAndRoles, kubernetesVersions[0], cnis[0], privateRegistriesNoAuth, rt.advancedOptions)
	require.NoError(rt.T(), err)
	clusterNameAuth, err := rt.testProvisionRKE1Cluster(subClient, provider, nodesAndRoles, kubernetesVersions[0], cnis[0], privateRegistriesAuth, rt.advancedOptions)
	require.NoError(rt.T(), err)

	clusterID, err := clusters.GetClusterIDByName(client, clusterNameNoAuth)
	require.NoError(rt.T(), err)

	rt.clusterNoAuthID = clusterID
	rt.clusterNoAuthRegistryHost = registryDisabledFqdn

	clusterID, err = clusters.GetClusterIDByName(client, clusterNameAuth)
	require.NoError(rt.T(), err)

	rt.clusterAuthID = clusterID
	rt.clusterAuthRegistryHost = registryEnabledFqdn

	rt.clusterLocalID = "local"
	rt.localClusterGlobalRegistryHost = globalRegistryFqdn

}

func (rt *RegistryTestSuite) testProvisionRKE1Cluster(client *rancher.Client, provider rke1.Provider, nodesAndRoles []nodepools.NodeRoles, kubeVersion, cni string, privateRegistries []management.PrivateRegistry, advancedOptions provisioning.AdvancedOptions) (string, error) {
	clusterName := namegen.AppendRandomString(provider.Name.String())
	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, "", client, advancedOptions)

	cluster.RancherKubernetesEngineConfig.PrivateRegistries = privateRegistries

	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(rt.T(), err)
	nodeTemplateResp, err := provider.NodeTemplateFunc(client)
	require.NoError(rt.T(), err)
	nodePool, err := nodepools.NodePoolSetup(client, nodesAndRoles, clusterResp.ID, nodeTemplateResp.ID)
	require.NoError(rt.T(), err)

	nodePoolName := nodePool.Name

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}
	watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(rt.T(), err)
	checkFunc := clusters.IsHostedProvisioningClusterReady
	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(rt.T(), err)

	assert.Equal(rt.T(), clusterName, clusterResp.Name)
	assert.Equal(rt.T(), nodePoolName, nodePool.Name)
	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(rt.T(), err)
	assert.NotEmpty(rt.T(), clusterToken)

	return clusterName, nil
}

func (rt *RegistryTestSuite) TestRegistryAllPods() {

	if rt.rancherUsesRegistry {
		havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterLocalID, rt.localClusterGlobalRegistryHost)
		require.NoError(rt.T(), err)
		assert.True(rt.T(), havePrefix)
	}

	havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterNoAuthID, rt.clusterNoAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), havePrefix)

	havePrefix, err = registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterAuthID, rt.clusterAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), havePrefix)

}

func (rt *RegistryTestSuite) TestStatusAllPods() {
	podResults, podErrors := pods.StatusPods(rt.client, rt.clusterLocalID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

	podResults, podErrors = pods.StatusPods(rt.client, rt.clusterNoAuthID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

	podResults, podErrors = pods.StatusPods(rt.client, rt.clusterAuthID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
