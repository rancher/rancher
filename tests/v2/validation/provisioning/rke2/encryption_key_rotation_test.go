package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type RKE2EncryptionKeyRotationTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	config      *provisioning.Config
	clusterName string
	namespace   string
}

const totalSecrets = 10000

var phases = []rkev1.RotateEncryptionKeysPhase{
	rkev1.RotateEncryptionKeysPhasePrepare,
	rkev1.RotateEncryptionKeysPhasePostPrepareRestart,
	rkev1.RotateEncryptionKeysPhaseRotate,
	rkev1.RotateEncryptionKeysPhasePostRotateRestart,
	rkev1.RotateEncryptionKeysPhaseReencrypt,
	rkev1.RotateEncryptionKeysPhasePostReencryptRestart,
	rkev1.RotateEncryptionKeysPhaseDone,
}

func (r *RKE2EncryptionKeyRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2EncryptionKeyRotationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.config = new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, r.config)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clusterName = r.client.RancherConfig.ClusterName
	r.namespace = r.client.RancherConfig.ClusterName
}

func provisionEnvironment(t *testing.T, client *rancher.Client, prefix string, provider Provider, version string, nodesAndRoles []machinepools.NodeRoles, credential *cloudcredentials.CloudCredential, psact string, advancedOptions provisioning.AdvancedOptions) string {
	clusterName := namegen.AppendRandomString(fmt.Sprintf("%s-%s", prefix, provider.Name))
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(t, err)

	machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "calico", credential.ID, version, psact, machinePools, advancedOptions)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(t, err)

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	err = wait.WatchWait(result, clusters.IsProvisioningClusterReady)
	require.NoError(t, err)
	assert.Equal(t, clusterName, clusterResp.ObjectMeta.Name)

	return clusterResp.ID
}

func rotateEncryptionKeys(t *testing.T, client *rancher.Client, steveID string, generation int64, timeout time.Duration) {
	t.Logf("Applying encryption key rotation generation %d for cluster %s", generation, steveID)

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	cluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(steveID)
	require.NoError(t, err)

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(t, err)

	updatedCluster := *cluster

	clusterSpec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
		Generation: generation,
	}

	updatedCluster.Spec = *clusterSpec

	cluster, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(cluster, updatedCluster)
	require.NoError(t, err)

	for _, phase := range phases {
		err = kwait.Poll(10*time.Second, timeout, IsAtLeast(t, client, namespace, cluster.ObjectMeta.Name, phase))
		require.NoError(t, err)
	}

	clusterWait, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	err = wait.WatchWait(clusterWait, clusters.IsProvisioningClusterReady)
	require.NoError(t, err)

	t.Logf("Successfully completed encryption key rotation for %s", cluster.ObjectMeta.Name)
}

func createSecretsForCluster(t *testing.T, client *rancher.Client, steveID string, scale int) {
	t.Logf("Creating %d secrets in namespace default for encryption key rotation", scale)

	_, clusterName, found := strings.Cut(steveID, "/")
	require.True(t, found)

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	secretResource, err := kubeapi.ResourceForClient(client, clusterID, "default", secrets.SecretGroupVersionResource)
	require.NoError(t, err)

	for i := 0; i < scale; i++ {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("encryption-key-rotation-test-%d-", i),
			},
			Data: map[string][]byte{
				"key": []byte(namegen.RandStringLower(5)),
			},
		}
		_, err = secrets.CreateSecret(secretResource, secret)
		require.NoError(t, err)
	}
}

func (r *RKE2EncryptionKeyRotationTestSuite) TestEncryptionKeyRotation() {
	for _, providerName := range r.config.Providers {
		for _, kubernetesVersion := range r.config.RKE2KubernetesVersions {
			// cleanup resources inside the for loop to prevent leaking
			subSession := r.session.NewSession()

			provider := CreateProvider(providerName)

			client, err := r.client.WithSession(subSession)
			require.NoError(r.T(), err)

			cloudCredential, err := provider.CloudCredFunc(client)
			require.NoError(r.T(), err)

			// provisioning is not considered part of the test
			id := provisionEnvironment(r.T(), client, r.clusterName, provider, kubernetesVersion, r.config.NodesAndRoles, cloudCredential, r.config.PSACT, r.config.AdvancedOptions)

			name := fmt.Sprintf("%s/%s/%v", provider.Name, kubernetesVersion, r.config.NodesAndRoles)
			r.Run(name+"-new-cluster", func() {
				rotateEncryptionKeys(r.T(), client, id, 1, 10*time.Minute)
			})

			// create 10k secrets for stress test, takes ~30 minutes
			createSecretsForCluster(r.T(), client, id, totalSecrets)

			r.Run(name+"-stress-test", func() {
				rotateEncryptionKeys(r.T(), client, id, 2, 1*time.Hour) // takes ~45 minutes for HA
			})

			subSession.Cleanup()
		}
	}
}

func IsAtLeast(t *testing.T, client *rancher.Client, namespace, name string, phase rkev1.RotateEncryptionKeysPhase) kwait.ConditionFunc {
	return func() (ready bool, err error) {
		kubeRKEClient, err := client.GetKubeAPIRKEClient()
		require.NoError(t, err)

		controlPlane, err := kubeRKEClient.RKEControlPlanes(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		require.NoError(t, err)

		if controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed {
			t.Errorf("Encryption key rotation failed waiting to reach %s", phase)
			return ready, fmt.Errorf("encryption key rotation failed")
		}

		desiredPhase := -1
		currentPhase := -1

		for i, v := range phases {
			if v == phase {
				desiredPhase = i
			}
			if v == controlPlane.Status.RotateEncryptionKeysPhase {
				currentPhase = i
			}
			if desiredPhase != -1 && currentPhase != -1 {
				break
			}
		}

		if currentPhase < desiredPhase {
			return false, nil
		}

		t.Logf("Encryption key rotation successfully entered %s", phase)

		return true, nil
	}
}

func TestEncryptionKeyRotation(t *testing.T) {
	suite.Run(t, new(RKE2EncryptionKeyRotationTestSuite))
}
