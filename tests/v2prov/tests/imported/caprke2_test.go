package imported

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_SetE_CAPRKE2DockerOperations brings up a single-node CAPRKE2 cluster on the CAPI
// Docker infrastructure provider, lets Turtles auto-import it into Rancher as a v3 Cluster, then
// walks all three operations (ETCDSnapshotSave → ETCDSnapshotRestore → EncryptionKeyRotation)
// against the CAPI Cluster — NOT the auto-imported v3 cluster.
//
// The CAPRKE2 adapter is registered in pkg/operations/capi.go for the
// `cluster.x-k8s.io/v1beta2 Cluster` GVK, so the operation's `ClusterRef` must point at the CAPI
// Cluster. The mgmt v3 mirror produced by Turtles is only here to verify the import wiring
// works; the operations themselves never reference it.
//
// This test is gated by V2PROV_TEST_CAPRKE2=true because spinning up the three Turtles-managed
// CAPI providers (CAPRKE2 control plane, CAPRKE2 bootstrap, CAPI Docker infrastructure) and a
// Docker-backed cluster on top adds significant time and resource pressure to the v2prov suite.
// CI does not set the env var so the test no-ops there by default; see
// `scripts/provisioning-tests` for the install gate.
func Test_Operation_SetE_CAPRKE2DockerOperations(t *testing.T) {
	if os.Getenv("V2PROV_TEST_CAPRKE2") != "true" {
		t.Skip("V2PROV_TEST_CAPRKE2 not set; skipping CAPRKE2 + Docker operations test")
	}

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Single all-roles RKE2 control-plane node. Keeps the cluster shape minimal and avoids
	// MachineDeployment / worker pool complexity for the smoke test.
	fx, err := cluster.NewCAPRKE2Cluster(cs, cluster.CAPRKE2Options{
		NamePrefix: "v2prov-caprke2",
		Replicas:   1,
	})
	if err != nil {
		t.Fatalf("creating CAPRKE2 cluster: %v", err)
	}
	cluster.WaitForCAPRKE2Ready(t, cs, fx)

	capiClusterRef := fx.CAPIClusterRef()
	t.Logf("CAPI cluster ready: namespace=%s name=%s mgmtV3Name=%s", fx.Namespace, fx.ClusterName, fx.MgmtClusterName)

	// Downstream client used for the configmap proof-of-restore. Built once from the CAPI
	// kubeconfig secret; survives restore so long as the API server returns within our poll window.
	downstream, err := fx.DownstreamClient(cs)
	if err != nil {
		t.Fatalf("building downstream client: %v", err)
	}

	// --- ETCDSnapshotSave ---
	// First operation: plain snapshot save. We also capture the cutoff time so the
	// back-populate wait later filters out any pre-existing snapshot.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			// Random suffix avoids collisions when the test re-runs against a shared cluster.
			Name:      "caprke2-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10)),
			Namespace: "default",
		},
		Data: map[string]string{"test": "wow"},
	}
	if _, err := downstream.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		t.Fatalf("creating proof-of-restore configmap %s: %v", cm.Name, err)
	}

	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.Namespace, capiClusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	// 1 etcd node → 1 snapshot file. The back-populate watcher mirrors snapshot files into
	// rkev1.ETCDSnapshot CRs in the CAPI cluster's namespace (= controlPlane namespace).
	waitForSnapshots(t, cs, fx.ClusterName, snapshotsValidAfter, 1)

	// --- ETCDSnapshotRestore ---
	// The CAPI Docker provider names its first machine `<cluster>-control-plane-<rand>`; the
	// machine-plan label `rke.cattle.io/node-name` carries the in-cluster node name, which for
	// the rke2 init node is conventionally `<machine-name>`. We pass the empty string here to
	// match any node, because the single-machine cluster only produces one snapshot CR.
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.ClusterName, "", snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot %s (file=%s)", snapshot.Name, snapshot.SnapshotFile.Name)

	// Delete the configmap so the post-restore check is meaningful.
	if err := downstream.CoreV1().ConfigMaps("default").Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("deleting proof-of-restore configmap %s: %v", cm.Name, err)
	}

	restoreOp := RunETCDSnapshotRestoreOperationTest(t, cs, fx.Namespace, snapshot.SnapshotFile.Name, capiClusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	// Poll the configmap back into existence. The apiserver bounces during a restore so the first
	// few Get calls may transiently fail before settling.
	var (
		gotValue string
		getErr   error
	)
	for i := 0; i < 60; i++ {
		got, err := downstream.CoreV1().ConfigMaps("default").Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if err == nil {
			gotValue = got.Data["test"]
			getErr = nil
			if gotValue == "wow" {
				break
			}
		} else {
			getErr = err
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v", cm.Name, getErr)
	}
	if gotValue != "wow" {
		t.Fatalf("expected configmap %s value restored to %q, got %q", cm.Name, "wow", gotValue)
	}

	// --- EncryptionKeyRotation ---
	// Run last because it pauses + restarts the cluster, and post-rotation we have no further
	// proof-of-life check besides the operation reaching Succeeded.
	ekrOp := RunEncryptionKeyRotationOperationTest(t, cs, fx.Namespace, capiClusterRef)
	t.Logf("encryption key rotation operation %s/%s completed", ekrOp.Namespace, ekrOp.Name)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, ekrOp.Status.Phase)
}
