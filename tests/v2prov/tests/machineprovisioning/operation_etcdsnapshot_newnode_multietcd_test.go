package machineprovisioning

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_SetB_MP_EtcdSnapshotOperationsWithThreeEtcdNodesOnNewNode uses Minio as an object store to store S3 snapshots.
// It creates a 5 node machine provisioned cluster with 3 controlplane+etcd nodes and 2 workers, creates a configmap,
// takes a snapshot of the cluster, deletes the configmap, then scales down the controlplane/etcd nodes.
// It then creates a new etcd node and restores from local snapshot file, then scales the cluster back up to desired state.
func Test_Operation_SetB_MP_EtcdSnapshotOperationsWithThreeEtcdNodesOnNewNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	newNs, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	osInfo, err := objectstore.GetObjectStore(clients, newNs.Name, "store0", "s3snapshots")
	if err != nil {
		t.Fatal(err)
	}

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp-etcd-snapshot-conventional-arch-new-node",
			Namespace: newNs.Name,
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
						S3: &rkev1.ETCDSnapshotS3{
							Endpoint:            osInfo.Endpoint,
							EndpointCA:          osInfo.Cert,
							Bucket:              osInfo.Bucket,
							CloudCredentialName: osInfo.CloudCredentialName,
							Folder:              "testfolder",
						},
					},
				},
				MachinePools: []provisioningv1.RKEMachinePool{
					{
						ControlPlaneRole: true,
						EtcdRole:         true,
						Quantity:         &defaults.Three,
					},
					{
						WorkerRole: true,
						Quantity:   &defaults.Two,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
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

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "s3")
	assert.NotNil(t, snapshot)
	// Scale controlplane/etcd nodes to 0
	c, err = operations.Scale(clients, c, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating insane cluster", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "waiting for at least one control plane, etcd, and worker node to be registered"), nil
	})
	// Scale etcd nodes to 1
	c, err = operations.Scale(clients, c, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating restoration required", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "rkecontrolplane was already initialized but no etcd machines exist that have plans, indicating the etcd plane has been entirely replaced. Restoration from etcd snapshot is required."), nil
	})
	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 3)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
