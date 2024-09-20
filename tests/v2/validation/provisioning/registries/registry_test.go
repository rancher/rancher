//go:build validation

package registries

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	provisioning "github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/registries"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	corralRancherName      = "rancherha"
	corralAuthDisabledName = "registryauthdisabled"
	corralAuthEnabledName  = "registryauthenabled"
	corralEcr              = "corralecr"
	systemRegistry         = "system-default-registry"
	namespace              = "fleet-default"
)

type RegistryTestSuite struct {
	suite.Suite
	session                        *session.Session
	client                         *rancher.Client
	standardUserClient             *rancher.Client
	clusterLocalID                 string
	localClusterGlobalRegistryHost string
	rancherUsesRegistry            bool
	provisioningConfig             *provisioninginput.Config
	privateRegistriesAuth          []management.PrivateRegistry
	privateRegistriesNoAuth        []management.PrivateRegistry
	privateEcr                     []management.PrivateRegistry
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

	enabled := true
	var testuser = namegenerator.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(rt.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(rt.T(), err)

	rt.standardUserClient = standardUserClient

	corralConfig := corral.Configurations()
	registriesConfig := new(Registries)
	config.LoadConfig(RegistriesConfigKey, registriesConfig)

	err = corral.SetupCorralConfig(corralConfig.CorralConfigVars, corralConfig.CorralConfigUser, corralConfig.CorralSSHPath)
	require.NoError(rt.T(), err)
	configPackage := corral.PackagesConfig()

	globalRegistryFqdn := ""
	registryDisabledFqdn := ""
	registryEnabledUsername := ""
	registryEnabledPassword := ""
	registryEnabledFqdn := ""
	ecrRegistryFqdn := ""
	ecrRegistryAwsAccessKey := ""
	ecrRegistryAwsSecretKey := ""
	ecrRegistryPassword := ""

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
		ecrRegistryFqdn, err = corral.GetCorralEnvVar(corralEcr, "registry_ecr_fqdn")
		require.NoError(rt.T(), err)
		logrus.Infof("Registry ECR FQDN %s", ecrRegistryFqdn)
		ecrRegistryAwsAccessKey, err = corral.GetCorralEnvVar(corralEcr, "aws_access_key")
		require.NoError(rt.T(), err)
		logrus.Infof("Registry ECR Access Key %s", ecrRegistryAwsAccessKey)
		ecrRegistryAwsSecretKey, err = corral.GetCorralEnvVar(corralEcr, "aws_secret_key")
		require.NoError(rt.T(), err)
		logrus.Infof("Registry ECR Secret Key %s", ecrRegistryAwsSecretKey)
		ecrRegistryPassword, err = corral.GetCorralEnvVar(corralEcr, "registry_password")
		require.NoError(rt.T(), err)
		logrus.Infof("Registry ECR Password %s", ecrRegistryPassword)

	} else {
		logrus.Infof("Using Existing Registries because value of useRegistries is %t", useRegistries)
		registryDisabledFqdn = registriesConfig.ExistingNoAuthRegistryURL
		registryEnabledFqdn = registriesConfig.ExistingAuthRegistryInfo.URL
		registryEnabledUsername = registriesConfig.ExistingAuthRegistryInfo.Username
		registryEnabledPassword = registriesConfig.ExistingAuthRegistryInfo.Password
		ecrRegistryFqdn = registriesConfig.ECRRegistryConfig.URL
		ecrRegistryAwsAccessKey = registriesConfig.ECRRegistryConfig.AwsAccessKeyID
		ecrRegistryAwsSecretKey = registriesConfig.ECRRegistryConfig.AwsSecretAccessKey
		ecrRegistryPassword = registriesConfig.ECRRegistryConfig.Password
		logrus.Infof("Registry ECR FQDN %s", ecrRegistryFqdn)
		logrus.Infof("Registry ECR Access Key %s", ecrRegistryAwsAccessKey)
		logrus.Infof("Registry ECR Secret Key %s", ecrRegistryAwsSecretKey)
		logrus.Infof("Registry ECR Password %s", ecrRegistryPassword)
		logrus.Infof("RegistryNoAuth FQDN %s", registryDisabledFqdn)
		logrus.Infof("RegistryAuth Username %s", registryEnabledUsername)
		logrus.Infof("RegistryAuth Password %s", registryEnabledPassword)
		logrus.Infof("RegistryAuth FQDN %s", registryEnabledFqdn)
	}

	rt.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, rt.provisioningConfig)

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

	ECRCredentialPlugin := &management.ECRCredentialPlugin{
		AwsAccessKeyID:     ecrRegistryAwsAccessKey,
		AwsSecretAccessKey: ecrRegistryAwsSecretKey,
	}
	privateRegistry = management.PrivateRegistry{}
	privateRegistry.URL = ecrRegistryFqdn
	privateRegistry.IsDefault = true
	privateRegistry.Password = ecrRegistryPassword
	privateRegistry.User = "AWS"
	rt.privateEcr = append(rt.privateEcr, privateRegistry)
	rt.privateEcr[0].ECRCredentialPlugin = ECRCredentialPlugin
}

func (rt *RegistryTestSuite) TestRegistriesRKE() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.standardUserClient.WithSession(subSession)
	require.NoError(rt.T(), err)

	if rt.provisioningConfig.RKE1KubernetesVersions == nil {
		rke1Versions, err := kubernetesversions.ListK3SAllVersions(rt.client)
		require.NoError(rt.T(), err)

		rt.provisioningConfig.RKE1KubernetesVersions = rke1Versions
	}

	tests := []struct {
		registry []management.PrivateRegistry
		name     string
	}{
		{rt.privateRegistriesNoAuth, "RKE1 Registry No Auth "},
		{rt.privateRegistriesAuth, "RKE1 Registry Auth "},
		{rt.privateEcr, "RKE1 Registry ECR "},
	}

	for _, tt := range tests {
		for _, k8sVersion := range rt.provisioningConfig.RKE1KubernetesVersions {
			rt.Run(tt.name+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)

				testConfig.KubernetesVersion = k8sVersion
				testConfig.CNI = rt.provisioningConfig.CNIs[0]

				if testConfig.Registries == nil {
					testConfig.Registries = &provisioninginput.Registries{}
				}
				testConfig.Registries.RKE1Registries = tt.registry
				_, rke1Provider, _, _ := permutations.GetClusterProvider(permutations.RKE1ProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				nodeTemplate, err := rke1Provider.NodeTemplateFunc(subClient)
				require.NoError(rt.T(), err)

				clusterObject, err := provisioning.CreateProvisioningRKE1Cluster(subClient, *rke1Provider, testConfig, nodeTemplate)
				reports.TimeoutRKEReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyRKE1Cluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}

	if rt.rancherUsesRegistry {
		for _, k8sVersion := range rt.provisioningConfig.RKE1KubernetesVersions {
			rt.Run("rke1 local cluster registry "+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)

				testConfig.KubernetesVersion = k8sVersion
				testConfig.CNI = rt.provisioningConfig.CNIs[0]

				_, rke1Provider, _, _ := permutations.GetClusterProvider(permutations.RKE1ProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				nodeTemplate, err := rke1Provider.NodeTemplateFunc(subClient)
				require.NoError(rt.T(), err)

				clusterObject, err := provisioning.CreateProvisioningRKE1Cluster(subClient, *rke1Provider, testConfig, nodeTemplate)
				reports.TimeoutRKEReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyRKE1Cluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}

	podErrors := pods.StatusPods(rt.client, rt.clusterLocalID)
	assert.Empty(rt.T(), podErrors)
	registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterLocalID, rt.localClusterGlobalRegistryHost)
}

func (rt *RegistryTestSuite) TestRegistriesK3S() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.standardUserClient.WithSession(subSession)
	require.NoError(rt.T(), err)

	if rt.provisioningConfig.K3SKubernetesVersions == nil {
		k3sVersions, err := kubernetesversions.ListK3SAllVersions(rt.client)
		require.NoError(rt.T(), err)

		rt.provisioningConfig.K3SKubernetesVersions = k3sVersions
	}

	tests := []struct {
		registry string
		name     string
	}{
		{rt.privateRegistriesNoAuth[0].URL, "K3S Registry No Auth "},
		{rt.privateRegistriesAuth[0].URL, "K3S Registry Auth "},
	}

	for _, tt := range tests {
		for _, k8sVersion := range rt.provisioningConfig.K3SKubernetesVersions {
			rt.Run(tt.name+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)
				testConfig.KubernetesVersion = k8sVersion
				testConfig.CNI = rt.provisioningConfig.CNIs[0]

				testConfig = rt.configureRKE2K3SRegistry(tt.registry, testConfig)

				k3sProvider, _, _, _ := permutations.GetClusterProvider(permutations.K3SProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				clusterObject, err := provisioning.CreateProvisioningCluster(subClient, *k3sProvider, testConfig, nil)
				reports.TimeoutClusterReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyCluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}

	if rt.rancherUsesRegistry {
		for _, k8sVersion := range rt.provisioningConfig.K3SKubernetesVersions {
			rt.Run("k3s local cluster registry "+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)
				testConfig.KubernetesVersion = rt.provisioningConfig.K3SKubernetesVersions[0]
				testConfig.CNI = rt.provisioningConfig.CNIs[0]

				testConfig = rt.configureRKE2K3SRegistry(rt.localClusterGlobalRegistryHost, testConfig)

				k3sProvider, _, _, _ := permutations.GetClusterProvider(permutations.K3SProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				clusterObject, err := provisioning.CreateProvisioningCluster(subClient, *k3sProvider, testConfig, nil)
				reports.TimeoutClusterReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyCluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}

	podErrors := pods.StatusPods(rt.client, rt.clusterLocalID)
	assert.Empty(rt.T(), podErrors)
	registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterLocalID, rt.localClusterGlobalRegistryHost)
}

func (rt *RegistryTestSuite) TestRegistriesRKE2() {
	subSession := session.NewSession()
	defer subSession.Cleanup()

	subClient, err := rt.standardUserClient.WithSession(subSession)
	require.NoError(rt.T(), err)

	if rt.provisioningConfig.RKE2KubernetesVersions == nil {
		rke2Versions, err := kubernetesversions.ListRKE2AllVersions(rt.client)
		require.NoError(rt.T(), err)

		rt.provisioningConfig.RKE2KubernetesVersions = rke2Versions
	}

	tests := []struct {
		registry string
		name     string
	}{
		{rt.privateRegistriesNoAuth[0].URL, "RKE2 Registry No Auth "},
		{rt.privateRegistriesAuth[0].URL, "RKE2 Registry Auth "},
	}

	for _, tt := range tests {
		for _, k8sVersion := range rt.provisioningConfig.RKE2KubernetesVersions {
			rt.Run(tt.name+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)
				testConfig.KubernetesVersion = k8sVersion
				testConfig.CNI = rt.provisioningConfig.CNIs[0]
				testConfig = rt.configureRKE2K3SRegistry(tt.registry, testConfig)

				rke2Provider, _, _, _ := permutations.GetClusterProvider(permutations.RKE2ProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				clusterObject, err := provisioning.CreateProvisioningCluster(subClient, *rke2Provider, testConfig, nil)
				reports.TimeoutClusterReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyCluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}
	if rt.rancherUsesRegistry {
		for _, k8sVersion := range rt.provisioningConfig.RKE2KubernetesVersions {
			rt.Run("rke2 local cluster registry "+k8sVersion, func() {
				testConfig := clusters.ConvertConfigToClusterConfig(rt.provisioningConfig)
				testConfig.KubernetesVersion = rt.provisioningConfig.RKE2KubernetesVersions[0]
				testConfig.CNI = rt.provisioningConfig.CNIs[0]
				testConfig = rt.configureRKE2K3SRegistry(rt.localClusterGlobalRegistryHost, testConfig)

				rke2Provider, _, _, _ := permutations.GetClusterProvider(permutations.RKE2ProvisionCluster, (*testConfig.Providers)[0], rt.provisioningConfig)

				clusterObject, err := provisioning.CreateProvisioningCluster(subClient, *rke2Provider, testConfig, nil)
				reports.TimeoutClusterReport(clusterObject, err)
				require.NoError(rt.T(), err)

				provisioning.VerifyCluster(rt.T(), subClient, testConfig, clusterObject)
			})
		}
	}

	podErrors := pods.StatusPods(rt.client, rt.clusterLocalID)
	assert.Empty(rt.T(), podErrors)
	registries.CheckAllClusterPodsForRegistryPrefix(rt.client, rt.clusterLocalID, rt.localClusterGlobalRegistryHost)
}

func (rt *RegistryTestSuite) configureRKE2K3SRegistry(registryName string, testConfig *clusters.ClusterConfig) *clusters.ClusterConfig {
	testConfig.Registries = &provisioninginput.Registries{
		RKE2Registries: &rkev1.Registry{
			Configs: map[string]rkev1.RegistryConfig{
				registryName: {},
			},
		},
	}
	if registryName == rt.privateRegistriesAuth[0].URL {
		testConfig.Registries.RKE2Password = rt.privateRegistriesAuth[0].Password
		testConfig.Registries.RKE2Username = rt.privateRegistriesAuth[0].User
	}
	if testConfig.Advanced == nil {
		testConfig.Advanced = &provisioninginput.Advanced{}
	}
	testConfig.Advanced.MachineSelectors = &[]rkev1.RKESystemConfig{
		{
			Config: rkev1.GenericMap{
				Data: map[string]interface{}{
					"protect-kernel-defaults": false,
					"system-default-registry": registryName,
				},
			},
		},
	}
	logrus.Infof("returning registry")
	return testConfig
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
