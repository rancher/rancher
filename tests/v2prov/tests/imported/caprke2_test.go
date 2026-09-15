package imported

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// The four Test_Imported_Operation_SetE_CAPRKE2Docker* tests below exercise the same operations
// (CertificateRotation → ETCDSnapshotSave → ETCDSnapshotRestore → EncryptionKeyRotation) against
// progressively larger CAPRKE2 topologies:
//
//   - SingleServer      — 1 control-plane, 0 workers (smallest smoke)
//   - OneServerOneAgent — 1 control-plane, 1 worker  (mixed roles, minimum for MachineDeployment)
//   - ThreeServers      — 3 control-planes, 0 workers (multi-etcd, tests quorum during restore)
//   - ThreeServersThreeAgents — 3+3 (full-shape stress)
//
// Each test creates a fresh CAPI Docker cluster, lets Turtles auto-import it, and then walks the
// operation sequence. The CAPRKE2 adapter is registered in pkg/operations/capi.go for the
// `cluster.x-k8s.io/v1beta2 Cluster` GVK, so operation ClusterRefs point at the CAPI Cluster —
// NOT the mgmt v3 mirror.
//
// All four tests are LOCAL-DEV ONLY and gated by V2PROV_TEST_CAPRKE2=true. CI does not set the env
// var (the provisioning-tests workflow has no CAPRKE2 matrix entry). Local recipe:
//
//	make dev-env                     # k3d cluster on the `kind` docker network with docker.sock
//	# run Rancher locally (dev-scripts/quick, or your GoLand run target)
//	make install-caprke2-providers   # waits for Rancher/Turtles, then applies the provider set
//	V2PROV_TEST_CAPRKE2=true go test -v \
//	  -run '^Test_Imported_Operation_SetE_CAPRKE2Docker' \
//	  ./tests/v2prov/tests/imported/...
//
// See dev-scripts/dev-env and dev-scripts/install-caprke2-providers for the invariants.

// Test_Imported_Operation_SetE_CAPRKE2DockerOperations is the historical single-server smoke test. Kept
// as the smallest topology for fast local iteration.
func Test_Imported_Operation_SetE_CAPRKE2DockerOperations(t *testing.T) {
	runCAPRKE2OperationsTest(t, cluster.CAPRKE2Options{
		NamePrefix:          "v2prov-caprke2",
		Replicas:            1,
		UseSnapshotFileName: true,
	})
}

func Test_Imported_Operation_SetE_CAPRKE2DockerOperations_OneServerOneAgent(t *testing.T) {
	runCAPRKE2OperationsTest(t, cluster.CAPRKE2Options{
		NamePrefix:          "v2prov-caprke2-1s1a",
		Replicas:            1,
		WorkerReplicas:      1,
		UseSnapshotFileName: true,
	})
}

func Test_Imported_Operation_SetE_CAPRKE2DockerOperations_ThreeServers(t *testing.T) {
	runCAPRKE2OperationsTest(t, cluster.CAPRKE2Options{
		NamePrefix: "v2prov-caprke2-3s",
		Replicas:   3,
	})
}

func Test_Imported_Operation_SetE_CAPRKE2DockerOperations_ThreeServersThreeAgents(t *testing.T) {
	runCAPRKE2OperationsTest(t, cluster.CAPRKE2Options{
		NamePrefix:     "v2prov-caprke2-3s3a",
		Replicas:       3,
		WorkerReplicas: 3,
	})
}

// runCAPRKE2OperationsTest brings up a CAPRKE2 cluster with the supplied topology, then walks
// certificate rotation, snapshot save/restore, and encryption-key rotation. The proof-of-restore
// check writes a ConfigMap to the downstream cluster before the save, deletes it after, and
// asserts it comes back after the restore.
func runCAPRKE2OperationsTest(t *testing.T, opts cluster.CAPRKE2Options) {
	if os.Getenv("V2PROV_TEST_CAPRKE2") != "true" {
		t.Skip("V2PROV_TEST_CAPRKE2 not set; skipping CAPRKE2 + Docker operations test (local-only)")
	}

	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx, err := cluster.NewCAPRKE2Cluster(cs, opts)
	if err != nil {
		t.Fatalf("creating CAPRKE2 cluster: %v", err)
	}
	cluster.WaitForCAPRKE2Ready(t, cs, fx)

	capiClusterRef := fx.CAPIClusterRef()
	t.Logf("CAPI cluster ready: namespace=%s name=%s mgmtV3Name=%s controlPlaneReplicas=%d workerReplicas=%d",
		fx.Namespace, fx.ClusterName, fx.MgmtClusterName, opts.Replicas, opts.WorkerReplicas)

	// --- CertificateRotation ---
	// Run first, before snapshot save/restore. Reaching Succeeded plus the downstream API check
	// below proves the operation completed and the CAPRKE2 cluster's API server recovered enough
	// to continue with normal Day2 operations — it does not collect certificate before/after
	// evidence, so it does not prove certificate metadata actually changed.
	// Use the explicit waiter because it also waits for the Beacon delegate chain to clear.
	// SnapshotSave runs next and must acquire that same Beacon.
	crOp := CreateCertificateRotationOp(t, cs, fx.Namespace, capiClusterRef)
	crOp = WaitForCertificateRotationSucceeded(t, cs, crOp, fx.Namespace, fx.ClusterName)
	t.Logf("certificate rotation operation %s/%s completed", crOp.Namespace, crOp.Name)

	// --- ETCDSnapshotSave ---
	// Confirm the API is usable before placing state in the snapshot. This ConfigMap is deleted
	// before restore and then read back afterward as the restore proof.
	cmName := "caprke2-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10))
	var createErr error
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		machineName := pickCAPRKE2InitMachineName(t, cs, fx)
		_, err := cluster.RunCAPRKE2Kubectl(ctx, machineName,
			"-n", "default", "create", "configmap", cmName, "--from-literal=test=wow")
		createErr = err
		// A previous attempt may have created the ConfigMap before its command timed out.
		return err == nil || strings.Contains(err.Error(), "AlreadyExists"), nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for downstream API recovery before creating proof-of-restore configmap %s: %v (last error: %v)", cmName, err, createErr)
	}
	t.Logf("downstream API recovered after certificate rotation; created proof-of-restore configmap %s", cmName)

	// Keep selection scoped to the snapshot created by this test run.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.Namespace, capiClusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	// For Turtles-imported CAPRKE2, snapshotbackpopulate writes snapshots to the management-cluster
	// mirror namespace. Expect one snapshot file per etcd (control-plane) node; workers do not run etcd.
	waitForSnapshots(t, cs, fx.MgmtClusterName, fx.MgmtClusterName, snapshotsValidAfter, int(opts.Replicas))

	// --- ETCDSnapshotRestore ---
	// Snapshots are labeled `rke.cattle.io/node-name = <CAPI Machine name>` (for CAPRKE2 the
	// in-cluster node name matches the CAPI Machine name). List the CAPI Machines carrying the
	// control-plane label and use the first as the init-node identifier. Restore replays from a
	// single etcd node's snapshot, so any one control-plane machine is a valid pick.
	initMachineName := pickCAPRKE2InitMachineName(t, cs, fx)
	t.Logf("using CAPI machine %s as init-node identifier for snapshot lookup", initMachineName)
	// Select the matching snapshot from the Turtles management-cluster mirror namespace.
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.MgmtClusterName, fx.MgmtClusterName, initMachineName, snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot %s (file=%s)", snapshot.Name, snapshot.SnapshotFile.Name)

	// Delete the configmap so the post-restore check is meaningful.
	machineName := pickCAPRKE2InitMachineName(t, cs, fx)
	if _, err := cluster.RunCAPRKE2Kubectl(cs.Ctx, machineName,
		"-n", "default", "delete", "configmap", cmName, "--ignore-not-found"); err != nil {
		t.Fatalf("deleting proof-of-restore configmap %s: %v", cmName, err)
	}

	// Single-server CAPRKE2 clusters restore against the raw on-disk snapshot file name;
	// multi-node clusters restore against the ETCDSnapshot CR name. See CAPRKE2Options.UseSnapshotFileName.
	snapshotRef := snapshot.Name
	if opts.UseSnapshotFileName {
		snapshotRef = snapshot.SnapshotFile.Name
	}
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, cs, fx.Namespace, snapshotRef, capiClusterRef)
	t.Logf("snapshot restore operation %s/%s completed (snapshotRef=%s)", restoreOp.Namespace, restoreOp.Name, snapshotRef)

	// Poll the configmap back into existence. The apiserver bounces during a restore so the first
	// few Get calls may transiently fail before settling.
	var gotValue string
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		machineName := pickCAPRKE2InitMachineName(t, cs, fx)
		output, err := cluster.RunCAPRKE2Kubectl(ctx, machineName,
			"-n", "default", "get", "configmap", cmName, "-o", "jsonpath={.data.test}")
		if err != nil {
			return false, nil
		}
		gotValue = strings.TrimSpace(string(output))
		return gotValue == "wow", nil
	})
	if err != nil {
		t.Fatalf("get configmap %s did not restore to %q after restore: %v (last value: %q)", cmName, "wow", err, gotValue)
	}

	// --- EncryptionKeyRotation ---
	// Run last because it pauses + restarts the cluster, and post-rotation we have no further
	// proof-of-life check besides the operation reaching Succeeded.
	ekrOp := RunEncryptionKeyRotationOperationTest(t, cs, fx.Namespace, capiClusterRef)
	t.Logf("encryption key rotation operation %s/%s completed", ekrOp.Namespace, ekrOp.Name)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, ekrOp.Status.Phase)
}

// pickCAPRKE2InitMachineName returns the name of the newest control-plane CAPI Machine in the
// cluster. "Newest" matters after restore/EKR churn: earlier machines may have been rolled and
// their snapshot CRs may no longer exist, so picking the freshest one gives the most reliable
// snapshot lookup. Fails the test if no control-plane machine is found.
func pickCAPRKE2InitMachineName(t *testing.T, cs *clients.Clients, fx *cluster.CAPRKE2Fixture) string {
	t.Helper()
	machines, err := cs.CAPI.Machine().List(fx.Namespace, metav1.ListOptions{
		LabelSelector: capiv1beta2.ClusterNameLabel + "=" + fx.ClusterName + "," + capiv1beta2.MachineControlPlaneLabel,
	})
	if err != nil {
		t.Fatalf("listing CAPI control-plane machines for cluster %s/%s: %v", fx.Namespace, fx.ClusterName, err)
	}
	if len(machines.Items) == 0 {
		t.Fatalf("no CAPI control-plane machines found for cluster %s/%s", fx.Namespace, fx.ClusterName)
	}
	sort.Slice(machines.Items, func(i, j int) bool {
		return machines.Items[i].CreationTimestamp.After(machines.Items[j].CreationTimestamp.Time)
	})
	return machines.Items[0].Name
}
