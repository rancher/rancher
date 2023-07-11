package k3s

import (
	"context"
	"fmt"
	"testing"

	kubeProvisioning "github.com/rancher/rancher/tests/framework/clients/provisioning"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	provisioningV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KdmChecksTestSuite struct {
	suite.Suite
	session               *session.Session
	client                *rancher.Client
	ns                    string
	k3skubernetesVersions []string
	cnis                  []string
	providers             []string
	nodesAndRoles         []machinepools.NodeRoles
	advancedOptions       provisioning.AdvancedOptions
}

const (
	defaultNamespace = "default"
)

func (k *KdmChecksTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *KdmChecksTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	k.ns = defaultNamespace

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	k.k3skubernetesVersions = clustersConfig.K3SKubernetesVersions

	k.cnis = clustersConfig.CNIs
	k.providers = clustersConfig.Providers
	k.nodesAndRoles = clustersConfig.NodesAndRoles
	k.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client
}

func (k *KdmChecksTestSuite) TestK3SK8sVersions() {
	logrus.Infof("checking for valid k8s versions..")
	require.GreaterOrEqual(k.T(), len(k.k3skubernetesVersions), 1)
	// fetching all available k8s versions from rancher
	releasedK8sVersions, _ := kubernetesversions.ListK3SAllVersions(k.client)
	logrus.Info("expected k8s versions : ", k.k3skubernetesVersions)
	logrus.Info("k8s versions available on rancher server : ", releasedK8sVersions)
	for _, expectedK8sVersion := range k.k3skubernetesVersions {
		require.Contains(k.T(), releasedK8sVersions, expectedK8sVersion)
	}
}

func (k *KdmChecksTestSuite) TestProvisioningSingleNodeK3SClusters() {
	require.GreaterOrEqual(k.T(), len(k.providers), 1)

	subSession := k.session.NewSession()
	defer subSession.Cleanup()

	client, err := k.client.WithSession(subSession)
	require.NoError(k.T(), err)

	kubeProvisioningClient, err := k.client.GetKubeAPIProvisioningClient()
	require.NoError(k.T(), err)

	providerName := k.providers[0]
	provider := CreateProvider(providerName)
	nodeRoles := k.nodesAndRoles

	clusterNames := []string{}
	clusterResps := []*provisioningV1.SteveAPIObject{}
	k8sVersions := []string{}

	for _, k8sVersion := range k.k3skubernetesVersions {

		clusterName := namegen.AppendRandomString(provider.Name.String())

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(k.T(), err)

		generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
		machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, k.ns)
		machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
		require.NoError(k.T(), err)

		machinePools := machinepools.RKEMachinePoolSetup(nodeRoles, machineConfigResp)
		cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, k.ns, "", cloudCredential.ID, k8sVersion, "", machinePools, k.advancedOptions)

		logrus.Info("provisioning " + k8sVersion + " cluster..")

		clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
		require.NoError(k.T(), err)

		clusterNames = append(clusterNames, clusterName)
		clusterResps = append(clusterResps, clusterResp)
		k8sVersions = append(k8sVersions, cluster.Spec.KubernetesVersion)
	}

	k.checkClustersReady(k.client, kubeProvisioningClient, clusterResps, clusterNames, k8sVersions)
}

func (k *KdmChecksTestSuite) checkClustersReady(client *rancher.Client, kubeProvisioningClient *kubeProvisioning.Client, clusterResps []*provisioningV1.SteveAPIObject, clusterNames []string, k8sVersions []string) {
	for i, clusterResp := range clusterResps {
		logrus.Info("waiting for cluster ", clusterResp.Name, " to be up..")
		result, err := kubeProvisioningClient.Clusters(k.ns).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + clusterResp.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		require.NoError(k.T(), err)
		checkFunc := clusters.IsProvisioningClusterReady
		err = wait.WatchWait(result, checkFunc)
		require.NoError(k.T(), err)

		assert.Equal(k.T(), clusterNames[i], clusterResp.Name)
		assert.Equal(k.T(), k.k3skubernetesVersions[i], k8sVersions[i])

		clusterID, err := clusters.GetClusterIDByName(client, clusterResp.Name)
		require.NoError(k.T(), err)

		podResults, podErrors := pods.StatusPods(client, clusterID)
		assert.NotEmpty(k.T(), podResults)
		assert.Empty(k.T(), podErrors)

	}
}

func TestPostKdmOutOfBandReleaseChecks(t *testing.T) {
	suite.Run(t, new(KdmChecksTestSuite))
}
