package custom

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace creates a custom 2 node cluster with a controlplane+worker and
// etcd node, creates a configmap, takes a snapshot of the cluster, deletes the configmap, then restores from snapshot.
// This validates that it is possible to restore a snapshot.
func Test_Operation_SetA_Custom_EtcdSnapshotCreationRestoreInPlace(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations-inplace",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd --node-name etcd-test-node", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "etcd-test-node")
	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 2)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
