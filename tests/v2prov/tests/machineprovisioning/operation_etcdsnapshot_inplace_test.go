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
	if err != nil {
		t.Fatal(err)
	}
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
					Registries:            &rkev1.Registry{},
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

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, machines.Items[0].Status.NodeRef.Name)

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2, capr.RestoreRKEConfigAll)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2, capr.RestoreRKEConfigKubernetesVersion)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2, capr.RestoreRKEConfigNone)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)
}
