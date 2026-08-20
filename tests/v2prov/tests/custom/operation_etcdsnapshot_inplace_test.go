package custom

import (
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace verifies each RKE configuration
// restore mode on a custom cluster, including etcd data, KubernetesVersion, and AdditionalManifest.
func Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations-inplace",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
					},
				},
			},
		},
	})
	require.NoError(t, err)

	command, err := cluster.CustomCommand(clients, c)
	require.NoError(t, err)

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	require.NoError(t, err)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd --node-name etcd-test-node", map[string]string{"custom-cluster-name": c.Name}, nil)
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}

	// Restore newest to oldest because restoring etcd removes ETCDSnapshot resources created
	// after the selected snapshot.
	snapshots := operations.RunSnapshotCreateTests(t, clients, c, cm, "etcd-test-node", 3)
	require.Len(t, snapshots, 3, "expected 3 snapshots to be created")

	snapshotK8sVersion := operations.SnapshotKubernetesVersion(t, snapshots[2])
	alternateK8sVersion := operations.AlternateKubernetesVersionForSnapshot(t, snapshotK8sVersion)

	operations.SetClusterAdditionalManifest(t, clients, c, "modified-for-restore-test")

	// Restore all RKE configuration from snapshot metadata.
	operations.DeleteConfigMapBeforeRestore(t, clients, c, cm)
	operations.RunSnapshotRestoreTestWithRKEConfig(t, clients, c, snapshots[2].Name, cm, 2, rkev1.RestoreRKEConfigAll, alternateK8sVersion)
	latestC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigAll should revert AdditionalManifest")
	assert.Equal(t, snapshotK8sVersion, latestC.Spec.KubernetesVersion, "RestoreRKEConfigAll should restore KubernetesVersion from snapshot")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	operations.SetClusterAdditionalManifest(t, clients, c, "modified-for-kv-restore-test")

	// Restore only KubernetesVersion from snapshot metadata.
	operations.DeleteConfigMapBeforeRestore(t, clients, c, cm)
	operations.RunSnapshotRestoreTestWithRKEConfig(t, clients, c, snapshots[1].Name, cm, 2, rkev1.RestoreRKEConfigKubernetesVersion, alternateK8sVersion)
	latestC, err = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "modified-for-kv-restore-test", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigKubernetesVersion should not revert AdditionalManifest")
	assert.Equal(t, snapshotK8sVersion, latestC.Spec.KubernetesVersion, "RestoreRKEConfigKubernetesVersion should restore KubernetesVersion from snapshot")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	// Restore etcd data without restoring RKE configuration.
	operations.DeleteConfigMapBeforeRestore(t, clients, c, cm)
	operations.RunSnapshotRestoreTestWithRKEConfig(t, clients, c, snapshots[0].Name, cm, 2, rkev1.RestoreRKEConfigNone, "")
	latestC, err = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "modified-for-kv-restore-test", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigNone should not revert AdditionalManifest")
	assert.Equal(t, snapshotK8sVersion, latestC.Spec.KubernetesVersion, "RestoreRKEConfigNone should preserve KubernetesVersion")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
