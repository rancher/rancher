package permutations

import (
	"strings"

	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/componentchecks"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	RKE2CustomCluster    = "rke2Custom"
	RKE2ProvisionCluster = "rke2"
	RKE2AirgapCluster    = "rke2Airgap"
	K3SCustomCluster     = "k3sCustom"
	K3SProvisionCluster  = "k3s"
	K3SAirgapCluster     = "k3sAirgap"
	RKE1CustomCluster    = "rke1Custom"
	RKE1ProvisionCluster = "rke1"
	RKE1AirgapCluster    = "rke1Airgap"
	CorralProvider       = "corral"
)

// RunTestPermutations runs through all relevant perumutations in a given config file, including node providers, k8s versions, and CNIs
func RunTestPermutations(s *suite.Suite, testNamePrefix string, client *rancher.Client, provisioningConfig *provisioninginput.Config, clusterType string, hostnameTruncation []machinepools.HostnameTruncation, corralPackages *corral.CorralPackages) {
	var name string
	var providers []string
	var testClusterConfig *clusters.ClusterConfig
	var err error

	testSession := session.NewSession()
	defer testSession.Cleanup()
	client, err = client.WithSession(testSession)
	require.NoError(s.T(), err)

	if strings.Contains(clusterType, "Custom") {
		providers = provisioningConfig.NodeProviders
	} else if strings.Contains(clusterType, "Airgap") {
		providers = []string{"Corral"}
	} else {
		providers = provisioningConfig.Providers
	}
	for _, nodeProviderName := range providers {
		nodeProvider, rke1Provider, customProvider, kubeVersions := GetClusterProvider(clusterType, nodeProviderName, provisioningConfig)
		for _, kubeVersion := range kubeVersions {
			for _, cni := range provisioningConfig.CNIs {
				testClusterConfig = clusters.ConvertConfigToClusterConfig(provisioningConfig)
				testClusterConfig.CNI = cni
				name = testNamePrefix + " Node Provider: " + nodeProviderName + " Kubernetes version: " + kubeVersion + " cni: " + cni
				s.Run(name, func() {
					switch clusterType {
					case RKE2ProvisionCluster, K3SProvisionCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, err := provisioning.CreateProvisioningCluster(client, *nodeProvider, testClusterConfig, hostnameTruncation)

						require.NoError(s.T(), err)
						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1ProvisionCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						nodeTemplate, err := rke1Provider.NodeTemplateFunc(client)
						require.NoError(s.T(), err)

						clusterObject, err := provisioning.CreateProvisioningRKE1Cluster(client, *rke1Provider, testClusterConfig, nodeTemplate)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE2CustomCluster, K3SCustomCluster:
						testClusterConfig.KubernetesVersion = kubeVersion

						clusterObject, err := provisioning.CreateProvisioningCustomCluster(client, customProvider, testClusterConfig)
						require.NoError(s.T(), err)

						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1CustomCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, nodes, err := provisioning.CreateProvisioningRKE1CustomCluster(client, customProvider, testClusterConfig)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, clusterObject)
						etcdVersion, err := componentchecks.CheckETCDVersion(client, nodes, clusterObject.ID)
						require.NoError(s.T(), err)
						require.NotEmpty(s.T(), etcdVersion)

					// airgap currently uses corral to create nodes and register with rancher
					case RKE2AirgapCluster, K3SAirgapCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, err := provisioning.CreateProvisioningAirgapCustomCluster(client, testClusterConfig, corralPackages)
						require.NoError(s.T(), err)

						provisioning.VerifyCluster(s.T(), client, testClusterConfig, clusterObject)

					case RKE1AirgapCluster:
						testClusterConfig.KubernetesVersion = kubeVersion
						clusterObject, err := provisioning.CreateProvisioningRKE1AirgapCustomCluster(client, testClusterConfig, corralPackages)
						require.NoError(s.T(), err)

						provisioning.VerifyRKE1Cluster(s.T(), client, testClusterConfig, clusterObject)

					default:
						s.T().Fatalf("Invalid cluster type: %s", clusterType)
					}
				})
			}
		}
	}
}

func GetClusterProvider(clusterType string, nodeProviderName string, provisioningConfig *provisioninginput.Config) (*provisioning.Provider, *provisioning.RKE1Provider, *provisioning.ExternalNodeProvider, []string) {
	var nodeProvider provisioning.Provider
	var rke1NodeProvider provisioning.RKE1Provider
	var customProvider provisioning.ExternalNodeProvider
	var kubeVersions []string

	switch clusterType {
	case RKE2ProvisionCluster:
		nodeProvider = provisioning.CreateProvider(nodeProviderName)
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	case K3SProvisionCluster:
		nodeProvider = provisioning.CreateProvider(nodeProviderName)
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1ProvisionCluster:
		rke1NodeProvider = provisioning.CreateRKE1Provider(nodeProviderName)
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case RKE2CustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	case K3SCustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1CustomCluster:
		customProvider = provisioning.ExternalNodeProviderSetup(nodeProviderName)
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case K3SAirgapCluster:
		kubeVersions = provisioningConfig.K3SKubernetesVersions
	case RKE1AirgapCluster:
		kubeVersions = provisioningConfig.RKE1KubernetesVersions
	case RKE2AirgapCluster:
		kubeVersions = provisioningConfig.RKE2KubernetesVersions
	default:
		panic("Cluster type not found")
	}
	return &nodeProvider, &rke1NodeProvider, &customProvider, kubeVersions
}
