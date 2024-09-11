//go:build (validation || infra.rke2k3s || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !sanity && !extended

package v2prov

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/secrets"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/kubeapi"
	"github.com/rancher/shepherd/extensions/vai"
	"github.com/rancher/shepherd/pkg/environmentflag"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type V2ProvEncryptionKeyRotationTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	clusterName string
}

const (
	namespace    = "fleet-default"
	totalSecrets = 10000
)

var phases = []rkev1.RotateEncryptionKeysPhase{
	rkev1.RotateEncryptionKeysPhasePrepare,
	rkev1.RotateEncryptionKeysPhasePostPrepareRestart,
	rkev1.RotateEncryptionKeysPhaseRotate,
	rkev1.RotateEncryptionKeysPhasePostRotateRestart,
	rkev1.RotateEncryptionKeysPhaseReencrypt,
	rkev1.RotateEncryptionKeysPhasePostReencryptRestart,
	rkev1.RotateEncryptionKeysPhaseDone,
}

func (r *V2ProvEncryptionKeyRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *V2ProvEncryptionKeyRotationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clusterName = r.client.RancherConfig.ClusterName
}

func setEncryptSecret(t *testing.T, client *rancher.Client, steveID string) {
	t.Logf("Set secret encryption key for cluster %s", steveID)

	cluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(steveID)
	require.NoError(t, err)

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(t, err)

	updatedCluster := *cluster

	secretEncryption := clusterSpec.RKEConfig.MachineGlobalConfig.Data["secrets-encryption"]

	isEncryptionDisabled := secretEncryption == nil || secretEncryption == false
	if isEncryptionDisabled {
		clusterSpec.RKEConfig.MachineGlobalConfig.Data["secrets-encryption"] = true

		updatedCluster.Spec = *clusterSpec

		cluster, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(cluster, updatedCluster)
		require.NoError(t, err)

		err = clusters.WatchAndWaitForCluster(client, steveID)
		require.NoError(t, err)

		t.Logf("Successfully set secret encryption key for %s", cluster.ObjectMeta.Name)
	}
}

func rotateEncryptionKeys(t *testing.T, client *rancher.Client, steveID string, timeout time.Duration) {
	t.Logf("Applying encryption key rotation for cluster %s", steveID)

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	cluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(steveID)
	require.NoError(t, err)

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	require.NoError(t, err)

	updatedCluster := *cluster

	rotateEncryptionKeys := clusterSpec.RKEConfig.RotateEncryptionKeys

	if rotateEncryptionKeys == nil {
		clusterSpec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
			Generation: 0,
		}
	}

	nextGeneration := clusterSpec.RKEConfig.RotateEncryptionKeys.Generation + 1
	clusterSpec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
		Generation: nextGeneration,
	}

	updatedCluster.Spec = *clusterSpec

	cluster, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(cluster, updatedCluster)
	require.NoError(t, err)

	for _, phase := range phases {
		err = kwait.Poll(10*time.Second, timeout, isAtLeast(t, client, namespace, cluster.ObjectMeta.Name, phase))
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

func getClusterType(client *rancher.Client, clusterID string) (string, error) {
	cluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterID)
	if err != nil {
		return "", err
	}

	clusterDetails := new(apiv1.Cluster)
	err = v1.ConvertToK8sType(cluster, &clusterDetails)
	if err != nil {
		return "", err
	}

	if strings.Contains(clusterDetails.Spec.KubernetesVersion, "rke2") {
		return "RKE2", nil
	} else {
		return "K3s", nil
	}

}

func (r *V2ProvEncryptionKeyRotationTestSuite) TestEncryptionKeyRotation() {
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	id, err := clusters.GetV1ProvisioningClusterByName(r.client, r.clusterName)
	require.NoError(r.T(), err)

	setEncryptSecret(r.T(), r.client, id)

	clusterType, err := getClusterType(r.client, id)
	require.NoError(r.T(), err)
	prefix := clusterType + "-encryption-key-rotation"

	r.Run(prefix, func() {
		rotateEncryptionKeys(r.T(), r.client, id, 10*time.Minute)
	})

	if r.client.Flags.GetValue(environmentflag.Long) {
		// create 10k secrets for stress test, takes ~30 minutes
		createSecretsForCluster(r.T(), r.client, id, totalSecrets)

		r.Run(prefix+"stress-test", func() {
			rotateEncryptionKeys(r.T(), r.client, id, 1*time.Hour) // takes ~45 minutes for HA
		})
	}
}

func (r *V2ProvEncryptionKeyRotationTestSuite) TestEncryptionKeyRotationWithVaiEnabled() {
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	id, err := clusters.GetV1ProvisioningClusterByName(r.client, r.clusterName)
	require.NoError(r.T(), err)

	setEncryptSecret(r.T(), r.client, id)
	err = vai.EnableVaiCaching(r.client)
	require.NoError(r.T(), err)

	clusterType, err := getClusterType(r.client, id)
	require.NoError(r.T(), err)
	prefix := clusterType + "-encryption-key-rotation-vai-enabled"

	r.Run(prefix, func() {
		rotateEncryptionKeys(r.T(), r.client, id, 10*time.Minute)
	})

	if r.client.Flags.GetValue(environmentflag.Long) {
		// create 10k secrets for stress test, takes ~30 minutes
		createSecretsForCluster(r.T(), r.client, id, totalSecrets)

		r.Run(prefix+"stress-test", func() {
			rotateEncryptionKeys(r.T(), r.client, id, 1*time.Hour) // takes ~45 minutes for HA
		})
	}
}

func isAtLeast(t *testing.T, client *rancher.Client, namespace, name string, phase rkev1.RotateEncryptionKeysPhase) kwait.ConditionFunc {
	return func() (ready bool, err error) {
		kubeRKEClient, err := client.GetKubeAPIRKEClient()
		if err != nil {
			return false, err
		}

		controlPlane, err := kubeRKEClient.RKEControlPlanes(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

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
	suite.Run(t, new(V2ProvEncryptionKeyRotationTestSuite))
}
