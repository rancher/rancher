package registries

import (
	"context"
	"fmt"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/registries"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/rancher/rancher/tests/v2/validation/provisioning/rke1"
	rke2k3s "github.com/rancher/rancher/tests/v2/validation/provisioning/rke2"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	corralRancherName      = "rancherha"
	corralAuthDisabledName = "registryauthdisabled"
	corralAuthEnabledName  = "registryauthenabled"
	systemRegistry         = "system-default-registry"
	namespace              = "fleet-default"
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
	clusterWithGlobalID            string
	localClusterGlobalRegistryHost string
	rancherUsesRegistry            bool
	advancedOptions                provisioning.AdvancedOptions
	cnis                           []string
	providers                      []string
	k3sKubernetesVersions          []string
	rkeKubernetesVersions          []string
	rke2KubernetesVersions         []string
	privateRegistriesAuth          []management.PrivateRegistry
	privateRegistriesNoAuth        []management.PrivateRegistry
	rkeNodesAndRoles               []nodepools.NodeRoles
	k3sRke2NodesAndRoles           []machinepools.NodeRoles
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
	rt.rkeKubernetesVersions = clustersConfig.RKE1KubernetesVersions
	rt.k3sKubernetesVersions = clustersConfig.K3SKubernetesVersions
	rt.rke2KubernetesVersions = clustersConfig.RKE2KubernetesVersions

	rt.cnis = clustersConfig.CNIs
	rt.rkeNodesAndRoles = clustersConfig.NodesAndRolesRKE1
	rt.k3sRke2NodesAndRoles = clustersConfig.NodesAndRoles
	rt.providers = clustersConfig.Providers
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
		var isSystemRegistrySet bool
		registry, err := client.Management.Setting.ByID(systemRegistry)
		require.NoError(rt.T(), err)

		if registry.Value != "" {
			isSystemRegistrySet = true
		}

		if useRegistries && isSystemRegistrySet {
			globalRegistryFqdn = registry.Value
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

	privateRegistry := management.PrivateRegistry{}
	privateRegistry.URL = registryDisabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = ""
	privateRegistry.User = ""
	rt.privateRegistriesNoAuth = append(rt.privateRegistriesNoAuth, privateRegistry)

	privateRegistry = management.PrivateRegistry{}
	privateRegistry.URL = registryEnabledFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = registryEnabledPassword
	privateRegistry.User = registryEnabledUsername
	rt.privateRegistriesAuth = append(rt.privateRegistriesAuth, privateRegistry)

	rt.clusterLocalID = "local"
	rt.localClusterGlobalRegistryHost = globalRegistryFqdn
}

func (rt *RegistryTestSuite) TestRegistriesRKE() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.client.WithSession(subSession)
	require.NoError(rt.T(), err)

	provider := rke1.CreateProvider(rt.providers[0])
	clusterNameNoAuth, err := rt.testProvisionRKE1Cluster(subClient, provider, rt.rkeNodesAndRoles, rt.rkeKubernetesVersions[0], rt.cnis[0], rt.privateRegistriesNoAuth, rt.advancedOptions)
	require.NoError(rt.T(), err)
	clusterNameAuth, err := rt.testProvisionRKE1Cluster(subClient, provider, rt.rkeNodesAndRoles, rt.rkeKubernetesVersions[0], rt.cnis[0], rt.privateRegistriesAuth, rt.advancedOptions)
	require.NoError(rt.T(), err)

	clusterID, err := clusters.GetClusterIDByName(rt.client, clusterNameNoAuth)
	require.NoError(rt.T(), err)
	rt.clusterNoAuthID = clusterID

	clusterID, err = clusters.GetClusterIDByName(rt.client, clusterNameAuth)
	require.NoError(rt.T(), err)
	rt.clusterAuthID = clusterID

	if rt.rancherUsesRegistry {
		clusterWithGlobal, err := rt.testProvisionRKE1Cluster(subClient, provider, rt.rkeNodesAndRoles, rt.rkeKubernetesVersions[0], rt.cnis[0], nil, rt.advancedOptions)
		require.NoError(rt.T(), err)
		clusterID, err := clusters.GetClusterIDByName(rt.client, clusterWithGlobal)
		require.NoError(rt.T(), err)
		rt.clusterWithGlobalID = clusterID
	}

	rt.testStatusAllPods()
	rt.testRegistryAllPods()
}

func (rt *RegistryTestSuite) TestRegistriesK3S() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.client.WithSession(subSession)
	require.NoError(rt.T(), err)
	rt.testAndProvisionRKE2K3SCluster(subClient, rt.k3sKubernetesVersions[0])
}

func (rt *RegistryTestSuite) TestRegistriesRKE2() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.client.WithSession(subSession)
	require.NoError(rt.T(), err)
	rt.testAndProvisionRKE2K3SCluster(subClient, rt.rke2KubernetesVersions[0])
}

func (rt *RegistryTestSuite) testAndProvisionRKE2K3SCluster(client *rancher.Client, kubernetesVersion string) {
	rt.clusterAuthRegistryHost = rt.privateRegistriesAuth[0].URL
	registryEnabledUsername := rt.privateRegistriesAuth[0].User
	registryEnabledPassword := rt.privateRegistriesAuth[0].Password

	rke2k3sProvider := rke2k3s.CreateProvider(rt.providers[0])
	rke2k3sClusterName := rt.testProvisionRKE2K3SCluster(client, rke2k3sProvider, rt.k3sRke2NodesAndRoles, kubernetesVersion, "", rt.clusterAuthRegistryHost, registryEnabledUsername, registryEnabledPassword, rt.advancedOptions)
	clusterID, err := clusters.GetClusterIDByName(rt.client, rke2k3sClusterName)
	require.NoError(rt.T(), err)
	rt.clusterAuthID = clusterID

	rt.clusterNoAuthRegistryHost = rt.privateRegistriesNoAuth[0].URL

	rke2k3sClusterName = rt.testProvisionRKE2K3SCluster(client, rke2k3sProvider, rt.k3sRke2NodesAndRoles, kubernetesVersion, "", rt.clusterNoAuthRegistryHost, "", "", rt.advancedOptions)
	clusterID, err = clusters.GetClusterIDByName(rt.client, rke2k3sClusterName)
	require.NoError(rt.T(), err)
	rt.clusterNoAuthID = clusterID

	if rt.rancherUsesRegistry {
		rke2k3sClusterName = rt.testProvisionRKE2K3SCluster(client, rke2k3sProvider, rt.k3sRke2NodesAndRoles, kubernetesVersion, "", rt.localClusterGlobalRegistryHost, "", "", rt.advancedOptions)
		clusterID, err = clusters.GetClusterIDByName(rt.client, rke2k3sClusterName)
		require.NoError(rt.T(), err)
		rt.clusterWithGlobalID = clusterID
	}

	rt.testStatusAllPods()
	rt.testRegistryAllPods()
}

func (rt *RegistryTestSuite) testProvisionRKE1Cluster(client *rancher.Client, provider rke1.Provider, nodesAndRoles []nodepools.NodeRoles, kubeVersion, cni string, privateRegistries []management.PrivateRegistry, advancedOptions provisioning.AdvancedOptions) (string, error) {
	clusterName := namegen.AppendRandomString(provider.Name.String())
	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, "", client, advancedOptions)

	if privateRegistries != nil {
		cluster.RancherKubernetesEngineConfig.PrivateRegistries = privateRegistries
	}

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

func (rt *RegistryTestSuite) testProvisionRKE2K3SCluster(client *rancher.Client, provider rke2k3s.Provider, nodesAndRoles []machinepools.NodeRoles, kubeVersion string, psact string, registryHostname string, registryUsername string, registryPassword string, advancedOptions provisioning.AdvancedOptions) string {
	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(rt.T(), err)

	clusterName := namegen.AppendRandomString(provider.Name.String())
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(rt.T(), err)

	machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)
	if registryUsername != "" && registryPassword != "" {
		steveClient, err := client.Steve.ProxyDownstream("local")
		require.NoError(rt.T(), err)
		secretName := fmt.Sprintf("priv-reg-sec-%s", clusterName)
		secretTemplate := secrets.NewSecretTemplate(secretName, namespace, map[string][]byte{
			"password": []byte(registryPassword),
			"username": []byte(registryUsername),
		},
			v1.SecretTypeBasicAuth,
		)

		registrySecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
		require.NoError(rt.T(), err)

		var machineSelectorConfig []rkev1.RKESystemConfig

		if len(registryHostname) > 0 {
			machineSelectorConfig = []rkev1.RKESystemConfig{
				{
					Config: rkev1.GenericMap{
						Data: map[string]interface{}{
							"protect-kernel-defaults": false,
							"system-default-registry": registryHostname,
						},
					},
				},
			}
		}

		var registries rkev1.Registry

		if registrySecret != nil {

			registries = rkev1.Registry{
				Configs: map[string]rkev1.RegistryConfig{
					registryHostname: {
						AuthConfigSecretName: registrySecret.Name,
					},
				},
			}

		}

		cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", cloudCredential.ID, kubeVersion, psact, machinePools, advancedOptions)
		cluster.Spec.RKEConfig.RKEClusterSpecCommon.MachineSelectorConfig = machineSelectorConfig
		cluster.Spec.RKEConfig.RKEClusterSpecCommon.Registries = &registries
		_, err = clusters.CreateK3SRKE2Cluster(client, cluster)
		require.NoError(rt.T(), err)
	} else {
		cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "", cloudCredential.ID, kubeVersion, psact, machinePools, advancedOptions)
		if rt.localClusterGlobalRegistryHost != registryHostname {
			var machineSelectorConfig []rkev1.RKESystemConfig
			if len(registryHostname) > 0 {
				machineSelectorConfig = []rkev1.RKESystemConfig{
					{
						Config: rkev1.GenericMap{
							Data: map[string]interface{}{
								"protect-kernel-defaults": false,
								"system-default-registry": registryHostname,
							},
						},
					},
				}
			}
			cluster.Spec.RKEConfig.RKEClusterSpecCommon.MachineSelectorConfig = machineSelectorConfig
		}
		_, err = clusters.CreateK3SRKE2Cluster(client, cluster)
		require.NoError(rt.T(), err)
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(rt.T(), err)
	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	require.NoError(rt.T(), err)

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(rt.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady
	err = wait.WatchWait(result, checkFunc)
	assert.NoError(rt.T(), err)

	return clusterName
}

func (rt *RegistryTestSuite) testRegistryAllPods() {

	if rt.rancherUsesRegistry {
		havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterLocalID, rt.localClusterGlobalRegistryHost)
		require.NoError(rt.T(), err)
		assert.True(rt.T(), havePrefix)

		havePrefix, err = registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterWithGlobalID, rt.localClusterGlobalRegistryHost)
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

func (rt *RegistryTestSuite) testStatusAllPods() {
	podResults, podErrors := pods.StatusPods(rt.client, rt.clusterLocalID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

	podResults, podErrors = pods.StatusPods(rt.client, rt.clusterNoAuthID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

	podResults, podErrors = pods.StatusPods(rt.client, rt.clusterAuthID)
	assert.NotEmpty(rt.T(), podResults)
	assert.Empty(rt.T(), podErrors)

	if rt.rancherUsesRegistry {
		podResults, podErrors = pods.StatusPods(rt.client, rt.clusterWithGlobalID)
		assert.NotEmpty(rt.T(), podResults)
		assert.Empty(rt.T(), podErrors)
	}
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
