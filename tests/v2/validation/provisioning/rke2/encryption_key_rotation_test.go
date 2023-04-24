package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
	"k8s.io/apimachinery/pkg/watch"
)

type RKE2EncryptionKeyRotationTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	config      *provisioning.Config
	clusterName string
	namespace   string
}

const scale = 10000

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

func (r *RKE2EncryptionKeyRotationTestSuite) TestEncryptionKeyRotationFreshCluster(provider Provider, kubeVersion string, nodesAndRoles []machinepools.NodeRoles, credential *cloudcredentials.CloudCredential) {
	name := fmt.Sprintf("Provider_%s/Kubernetes_Version_%s/Nodes_%v", provider.Name, kubeVersion, nodesAndRoles)
	r.Run(name, func() {
		testSession := session.NewSession()
		defer testSession.Cleanup()

		testSessionClient, err := r.client.WithSession(testSession)
		require.NoError(r.T(), err)

		clusterName := namegen.AppendRandomString(fmt.Sprintf("%s-%s", r.clusterName, provider.Name))
		generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
		machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

		machineConfigResp, err := testSessionClient.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
		require.NoError(r.T(), err)

		machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

		cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, "calico", credential.ID, kubeVersion, "", machinePools, r.config.AdvancedOptions)

		if strings.Contains(kubeVersion, "k3s") {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data["secrets-encryption"] = true
		}

		clusterResp, err := clusters.CreateK3SRKE2Cluster(testSessionClient, cluster)
		require.NoError(r.T(), err)

		kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
		require.NoError(r.T(), err)

		result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		require.NoError(r.T(), err)

		err = wait.WatchWait(result, clusters.IsProvisioningClusterReady)
		require.NoError(r.T(), err)
		assert.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)

		require.NoError(r.T(), r.rotateEncryptionKeys(clusterResp.ID, 1, defaults.WatchTimeoutSeconds))
		r.T().Logf("Successfully completed encryption key rotation for %s", name)

		r.T().Logf("Creating %d secrets in namespace default for encryption key rotation for %s", scale, name)

		clusterID, err := clusters.GetClusterIDByName(r.client, clusterName)
		require.NoError(r.T(), err)
		secretResource, err := kubeapi.ResourceForClient(r.client, clusterID, "default", secrets.SecretGroupVersionResource)
		require.NoError(r.T(), err)

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
			require.NoError(r.T(), err)
		}

		r.T().Logf("Successfully created %d secrets in namespace default for encryption key rotation for %s", scale, name)
		// encryption key rotation is capped at 5 secrets per second (10 every 2 seconds), so 10000 secrets will take
		// 2000 seconds which is ~33 minutes.
		require.NoError(r.T(), r.rotateEncryptionKeys(clusterResp.ID, 2, 60*60))
		r.T().Logf("Successfully completed second encryption key rotation for %s", name)
	})
}

func (r *RKE2EncryptionKeyRotationTestSuite) rotateEncryptionKeys(id string, generation, timeout int64) error {
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)

	cluster, err := r.client.Steve.SteveType(clusters.ProvisioningSteveResouceType).ByID(id)
	if err != nil {
		return err
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(r.T(), err)

	updatedCluster := *cluster

	clusterSpec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
		Generation: generation,
	}

	updatedCluster.Spec = *clusterSpec

	cluster, err = r.client.Steve.SteveType(clusters.ProvisioningSteveResouceType).Update(cluster, updatedCluster)
	if err != nil {
		return err
	}

	kubeRKEClient, err := r.client.GetKubeAPIRKEClient()
	require.NoError(r.T(), err)

	for _, phase := range phases {
		result, err := kubeRKEClient.RKEControlPlanes(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
			TimeoutSeconds: &timeout,
		})
		require.NoError(r.T(), err)

		err = wait.WatchWait(result, IsAtLeast(r.T(), phase))
		if err != nil {
			return err
		}
	}

	clusterWait, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(clusterWait, clusters.IsProvisioningClusterReady)
	if err != nil {
		return err
	}

	return nil
}

func (r *RKE2EncryptionKeyRotationTestSuite) TestEncryptionKeyRotation() {
	for _, providerName := range r.config.Providers {
		subSession := r.session.NewSession()

		provider := CreateProvider(providerName)

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)

		for _, kubernetesVersion := range r.config.RKE2KubernetesVersions {
			r.TestEncryptionKeyRotationFreshCluster(provider, kubernetesVersion, r.config.NodesAndRoles, cloudCredential)
		}

		subSession.Cleanup()
	}
}

func IsAtLeast(t *testing.T, phase rkev1.RotateEncryptionKeysPhase) wait.WatchCheckFunc {
	return func(event watch.Event) (ready bool, err error) {
		controlPlane := event.Object.(*rkev1.RKEControlPlane)

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
