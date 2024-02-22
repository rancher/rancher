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
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Operation_SetA_MP_EtcdSnapshotCreationRestoreInPlace(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mp-etcd-snapshot-operations-inplace",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
					},
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
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := clients.CAPI.Machine().List(c.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", capr.EtcdRoleLabel, "true")})
	if err != nil {
		t.Fatal(err)
	}

	if len(machines.Items) != 1 {
		t.Fatal(fmt.Errorf("length of etcd machines is not 1"))
	}

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
	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
