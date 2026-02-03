package machineprovisioning

import (
	"fmt"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Operation_SetA_MP_EtcdSnapshotCreationRestoreInPlace(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

	// Initialize empty structures to prevent nil pointer issues during deep equality checks in controllers
	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mp-etcd-snapshot-operations-inplace",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
					},
					MachineGlobalConfig:   rkev1.GenericMap{},
					MachineSelectorConfig: []rkev1.RKESystemConfig{},
					ChartValues:           rkev1.GenericMap{},
					UpgradeStrategy:       rkev1.ClusterUpgradeStrategy{},
					AdditionalManifest:    "",
					Networking:            &rkev1.Networking{},
				},
				MachinePools: []provisioningv1.RKEMachinePool{
					{
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
						RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
							Labels: map[string]string{
								"node-type": "etcd",
							},
						},
					},
					{
						EtcdRole: true,
						Quantity: &defaults.One,
					},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	machines, err := clients.CAPI.Machine().List(c.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", capr.EtcdRoleLabel, "true")})
	require.NoError(t, err)
	require.Len(t, machines.Items, 1, "expected exactly 1 etcd machine")
	assert.NotNil(t, machines.Items[0].Status.NodeRef)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}

	// Create 3 snapshots upfront - one for each restore type
	// This avoids issues where snapshots get deleted after RestoreRKEConfigAll
	snapshots := operations.RunSnapshotCreateTest(t, clients, c, cm, machines.Items[0].Status.NodeRef.Name, 3)
	require.Len(t, snapshots, 3, "expected 3 snapshots to be created")

	// Record original K8s version for post-restore validation
	originalK8sVersion := c.Spec.KubernetesVersion

	// Modify a cluster spec field to validate that restore modes properly revert (or preserve) spec changes
	operations.ModifyClusterAdditionalManifest(t, clients, c, "modified-for-restore-test")

	// RestoreRKEConfigAll: should revert both KubernetesVersion and spec fields to snapshot values
	operations.RunSnapshotRestoreTest(t, clients, c, snapshots[2].Name, cm, 2, rkev1.RestoreRKEConfigAll)
	latestC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigAll should revert AdditionalManifest")
	assert.Equal(t, originalK8sVersion, latestC.Spec.KubernetesVersion, "RestoreRKEConfigAll should restore KubernetesVersion")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	// Modify spec again before the next restore
	operations.ModifyClusterAdditionalManifest(t, clients, c, "modified-for-kv-restore-test")

	// RestoreRKEConfigKubernetesVersion: should only restore KubernetesVersion, not other spec fields
	operations.RunSnapshotRestoreTest(t, clients, c, snapshots[1].Name, cm, 2, rkev1.RestoreRKEConfigKubernetesVersion)
	latestC, err = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "modified-for-kv-restore-test", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigKubernetesVersion should not revert AdditionalManifest")
	assert.Equal(t, originalK8sVersion, latestC.Spec.KubernetesVersion, "RestoreRKEConfigKubernetesVersion should restore KubernetesVersion")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	// RestoreRKEConfigNone: should not revert any spec fields
	operations.RunSnapshotRestoreTest(t, clients, c, snapshots[0].Name, cm, 2, rkev1.RestoreRKEConfigNone)
	latestC, err = clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "modified-for-kv-restore-test", latestC.Spec.RKEConfig.AdditionalManifest, "RestoreRKEConfigNone should not revert AdditionalManifest")
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)
}
