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
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// Test_Operation_SetB_MP_EtcdSnapshotOperationsWithThreeEtcdNodesOnNewNode uses Minio as an object store to store S3 snapshots.
// It creates a 5 node machine provisioned cluster with 3 controlplane+etcd nodes and 2 workers, creates a configmap,
// takes a snapshot of the cluster, deletes the configmap, then scales down the controlplane/etcd nodes.
// The old init etcd Machine stays protected until a replacement joins through it, and then the cluster is restored from the S3 snapshot.
// After the restore, the cluster is scaled back up to its desired state.
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

	oldInit := findInitEtcdMachine(t, clients, c)

	// Scale controlplane/etcd nodes to 0
	c, err = operations.Scale(clients, c, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rkecontrolplane ready condition indicating the last etcd member is protected", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return strings.Contains(capr.Ready.GetMessage(&rkeControlPlane.Status), "last etcd member is protected"), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// The old init Machine must stay held (deleting, not removed) until the replacement takes over.
	held, err := clients.CAPI.Machine().Get(oldInit.Namespace, oldInit.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if held.DeletionTimestamp.IsZero() {
		t.Fatalf("old init etcd machine %s is not deleting", held.Name)
	}
	// Observation only: the non-init etcd Machines are not asserted on here.
	logEtcdMachines(t, clients, c, "after scale to 0")

	// Deleting Machines still count toward the MachineSet size, so scaling straight to 1 creates nothing.
	// Scale above the held etcd Machines to create one replacement, then scale back to 1 so the deleting Machines are removed first.
	ml, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}
	heldUIDs := map[types.UID]bool{}
	for i := range ml.Items {
		if ml.Items[i].Labels[capr.EtcdRoleLabel] == "true" {
			heldUIDs[ml.Items[i].UID] = true
		}
	}
	heldCount := len(heldUIDs)
	t.Logf("etcd machines still held before replacement: %d", heldCount)

	c, err = operations.Scale(clients, c, 0, int32(heldCount+1), false)
	if err != nil {
		t.Fatal(err)
	}

	var replacement *capi.Machine
	for deadline := time.Now().Add(10 * time.Minute); replacement == nil; time.Sleep(2 * time.Second) {
		if time.Now().After(deadline) {
			t.Fatal("no replacement etcd machine was created")
		}
		ml, err := cluster.Machines(clients, c)
		if err != nil {
			t.Fatal(err)
		}
		for i := range ml.Items {
			m := &ml.Items[i]
			if m.Labels[capr.EtcdRoleLabel] == "true" && m.DeletionTimestamp.IsZero() && !heldUIDs[m.UID] {
				replacement = m
				break
			}
		}
	}

	// Scale etcd nodes to 1
	c, err = operations.Scale(clients, c, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := waitForMachineGone(clients, oldInit, 15*time.Minute); err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	newInit := findInitEtcdMachine(t, clients, c)
	if newInit.UID == oldInit.UID {
		t.Fatal("the old init etcd machine is still the init node")
	}
	if newInit.UID != replacement.UID {
		t.Fatalf("the new init etcd machine %s is not the replacement %s", newInit.Name, replacement.Name)
	}
	logEtcdMachines(t, clients, c, "before restore")

	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 3)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

// logEtcdMachines logs each etcd Machine's name, init label and deletion state, without asserting on them.
func logEtcdMachines(t *testing.T, clients *clients.Clients, c *provisioningv1.Cluster, stage string) {
	t.Helper()
	ml, err := cluster.Machines(clients, c)
	if err != nil {
		t.Logf("%s: listing machines failed: %v", stage, err)
		return
	}
	for i := range ml.Items {
		m := &ml.Items[i]
		if m.Labels[capr.EtcdRoleLabel] != "true" {
			continue
		}
		initLabel := "unknown"
		if s, err := initProtectedPlanSecret(clients, m); err == nil {
			initLabel = s.Labels[capr.InitNodeLabel]
		}
		t.Logf("%s: etcd machine %s init=%q deleting=%t", stage, m.Name, initLabel, !m.DeletionTimestamp.IsZero())
	}
}
