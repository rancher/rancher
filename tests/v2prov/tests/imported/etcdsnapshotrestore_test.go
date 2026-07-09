package imported

import (
	"fmt"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotrestore"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
)

// configMapProof drives the standard "create CM → snapshot → delete CM → restore → CM is back"
// scenario inside the downstream cluster. Each restore test composes its own setup + hook
// behavior around these methods so the test body reads as the actual story, not boilerplate.
type configMapProof struct {
	name  string
	value string
}

// newConfigMapProof returns a configMapProof with a random suffix so concurrent test runs against a
// shared cluster don't collide on names.
func newConfigMapProof() *configMapProof {
	return &configMapProof{
		name:  "test-restore-cm-" + strings.ToLower(name.Hex(time.Now().String(), 10)),
		value: "wow",
	}
}

// create writes the ConfigMap inside the downstream cluster — this is the payload whose presence
// after restore proves the etcd snapshot was actually rolled back.
func (c *configMapProof) create(t *testing.T, fx *importedClusterFixture) {
	t.Helper()
	out, err := fx.execKubectl(t, fmt.Sprintf("kubectl create configmap %s --from-literal=test=%s", c.name, c.value))
	if err != nil {
		t.Fatalf("create configmap failed: %v\noutput: %s", err, out)
	}
}

// delete removes the ConfigMap before the restore so the post-restore assertion is meaningful — if
// we left it in place we'd be testing "still there" rather than "restored".
func (c *configMapProof) delete(t *testing.T, fx *importedClusterFixture) {
	t.Helper()
	out, err := fx.execKubectl(t, fmt.Sprintf("kubectl delete configmap %s", c.name))
	if err != nil {
		t.Fatalf("delete configmap failed: %v\noutput: %s", err, out)
	}
}

// assertRestored polls the ConfigMap until it's readable AND matches the expected value, then
// fails the test if neither condition is met within the timeout. The apiserver typically takes a
// while to start serving requests again after a restore, so the poll window has to be generous.
func (c *configMapProof) assertRestored(t *testing.T, fx *importedClusterFixture) {
	t.Helper()
	getCmd := fmt.Sprintf("kubectl get configmap %s -o jsonpath='{.data.test}'", c.name)
	var (
		got    string
		getErr error
	)
	for i := 0; i < 60; i++ {
		got, getErr = fx.execKubectl(t, getCmd)
		if getErr == nil && strings.TrimSpace(got) == c.value {
			return
		}
		time.Sleep(5 * time.Second)
	}
	if getErr != nil {
		t.Fatalf("get configmap %s failed after restore: %v\noutput: %s", c.name, getErr, got)
	}
	if strings.TrimSpace(got) != c.value {
		t.Fatalf("expected configmap %s value to be restored to %q, got %q", c.name, c.value, got)
	}
}

// runRestoreScenario factors out the per-test snapshot → delete → restore → verify sequence shared
// across the topology-specific tests. It takes the fixture, expected snapshot count (one per etcd
// node), and the back-populated snapshot node-name to restore from. The optional restoreFromName
// override lets callers restore by the rkev1.ETCDSnapshot CR name rather than the snapshot file
// name — useful for the 3-node-each-role topology where the back-populated CR carries owner refs.
func runRestoreScenario(
	t *testing.T,
	cs *clients.Clients,
	fx *importedClusterFixture,
	expectedSnapshotCount int,
	snapshotNodeName string,
	restoreByCRName bool,
) {
	t.Helper()

	cm := newConfigMapProof()
	cm.create(t, fx)

	// snapshotsValidAfter marks the cutoff time so a stale snapshot from a prior reconcile or
	// shared cluster run doesn't get picked up by the wait helpers. The 30-second backdating
	// compensates for clock skew between the test runner and the in-cluster controller.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)

	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, expectedSnapshotCount)

	// The snapshotbackpopulate controller mirrors downstream ETCDSnapshotFile resources into the
	// management cluster as rkev1.ETCDSnapshot CRs, in the namespace named for the cluster.
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotNodeName, snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}
	t.Logf("using snapshot %s (file=%s)", snapshot.Name, snapshot.SnapshotFile.Name)

	cm.delete(t, fx)

	// The 3-nodes-all-roles topology restores by snapshot CR name (which carries owner refs back
	// to a specific machine), whereas the single-node topology restores by the raw snapshot file
	// name. The controller resolves either form.
	restoreName := snapshot.SnapshotFile.Name
	if restoreByCRName {
		restoreName = snapshot.Name
	}
	restoreOp := RunETCDSnapshotRestoreOperationTest(t, cs, fx.ns.Name, restoreName, fx.clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restoreOp.Namespace, restoreOp.Name)

	cm.assertRestored(t, fx)
}

// Test_Operation_SetD_ImportedETCDSnapshotRestore brings up an imported single-node cluster,
// creates a ConfigMap inside the downstream cluster, takes a snapshot via ETCDSnapshotSave,
// deletes the ConfigMap, runs ETCDSnapshotRestore, then verifies the ConfigMap returns. This
// exercises the operations-controller-driven restore path end-to-end against an imported cluster
// (where there is no provisioning.cattle.io Cluster or RKEControlPlane to drive the restore via
// spec).
func Test_Operation_SetD_ImportedETCDSnapshotRestore(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-restore", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Single all-roles node: one etcd member → one snapshot. Restore from imported-init-0
	// (the init node has the leader's snapshot file).
	runRestoreScenario(t, cs, fx, 1, "imported-init-0", false)
}

// Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRoles is the same proof-of-restore as
// the single-node variant, but with 3 nodes each holding all three roles. Three etcd members
// produce three snapshot files (one per node). The restore is driven from imported-node-2 to
// deliberately exercise the non-init etcd path.
func Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesAllRoles(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-restore-3-nodes-all-roles", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 3},
	})

	// 3 etcd members → 3 snapshot files. Restore by the rkev1.ETCDSnapshot CR name so the
	// controller picks the right machine via the snapshot's owner reference.
	runRestoreScenario(t, cs, fx, 3, "imported-node-2", true)
}

// Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesOneEach is the same proof-of-restore but
// with three nodes each pinned to a single role (one etcd, one control-plane, one worker). The
// single etcd node means exactly one snapshot file even though the cluster has three nodes —
// covers the topology where roles are not collocated.
func Test_Operation_SetE_ImportedETCDSnapshotRestore3NodesOneEach(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-restore-3-nodes-1-each", []cluster.ImportedNodePool{
		{ETCD: true, Quantity: 1},
		{ControlPlane: true, Quantity: 1},
		{Worker: true, Quantity: 1},
	})

	// Single etcd node → one snapshot file. Restore by name to mirror the single-node behavior.
	runRestoreScenario(t, cs, fx, 1, "imported-init-0", true)
}

// Test_Operation_SetD_ImportedETCDSnapshotRestoreLifecycleHook walks an ETCDSnapshotRestore through
// every state-machine checkpoint by attaching a lifecycle-hook label for each step + the
// Succeeded phase. At each checkpoint the test:
//
//  1. Waits for the operation to land in the expected (phase, step) AND for the controller to push
//     the named delegate onto the beacon — proving the hook actually fired.
//  2. (Inspection point) The captured SnapshotRestoreCheckpoint exposes the latest op object and
//     beacon. For the restore path this is where a richer test would, for example: read the
//     machine-plan secrets to verify the shutdown plan contains the etcd-tombstone touch + tls
//     directory cleanup; override a node's plan-status secret to simulate a partial restore
//     failure; or assert that the snapshotbackpopulate controller has been quiesced behind the
//     beacon.
//  3. Clears the hook label and pops the delegate, releasing the controller to do the actual step
//     work before reaching the next gated checkpoint.
//
// Because the controller drives the actual restore between checkpoints, this test also verifies
// the proof-of-restore ConfigMap returns at the end — the hook framework should be transparent to
// the underlying operation's correctness.
func Test_Operation_SetD_ImportedETCDSnapshotRestoreLifecycleHook(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-restore-lifecycle-hook", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// Produce a snapshot we can restore from. The save here is the standard non-hooked flow —
	// the hook coverage is on restore only.
	cm := newConfigMapProof()
	cm.create(t, fx)

	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	saveOp := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	t.Logf("snapshot save operation %s/%s completed", saveOp.Namespace, saveOp.Name)
	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, 1)
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)
	if snapshot.SnapshotFile.Name == "" {
		t.Fatalf("back-populated snapshot %s has empty SnapshotFile.Name", snapshot.Name)
	}

	cm.delete(t, fx)

	// Restore steps in dispatch order. We hook every step plus the Succeeded phase so the test
	// walks the entire state machine. The InitialRestartCluster prefix and the (final)
	// RestartCluster prefix are intentionally distinct in the controller so delegates can target
	// either restart pass without gating the other.
	const (
		hookName     = "v2prov-e2e-test"
		delegateName = "v2prov-e2e-test-delegate"
	)
	stepHookKeys := []struct {
		name  string
		key   string
		phase opv1alpha1.OperationPhase
		step  opv1alpha1.ETCDSnapshotRestoreStep
	}{
		{"Preflight", etcdsnapshotrestore.PreflightStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepPreflight},
		{"Shutdown", etcdsnapshotrestore.ShutdownStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepShutdown},
		{"Restore", etcdsnapshotrestore.RestoreStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepRestore},
		{"PostRestorePodCleanup", etcdsnapshotrestore.PostRestorePodCleanupStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup},
		{"InitialRestartCluster", etcdsnapshotrestore.InitialRestartClusterStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster},
		{"PostRestoreNodeCleanup", etcdsnapshotrestore.PostRestoreNodeCleanupStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup},
		{"RestartCluster", etcdsnapshotrestore.RestartClusterStepHookLabelPrefix + hookName, opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotRestoreStepRestartCluster},
	}
	succeededHookKey := planv1alpha1.SucceededPhaseHookLabelPrefix + hookName

	// Attach every hook up front. Each prefix is scoped to a specific handler so they don't
	// interfere — the controller only consults the relevant prefix when it enters that
	// phase/step.
	labels := map[string]string{succeededHookKey: delegateName}
	for _, h := range stepHookKeys {
		labels[h.key] = delegateName
	}

	op := CreateETCDSnapshotRestoreOp(t, cs, fx.ns.Name, snapshot.SnapshotFile.Name, fx.clusterRef, WithRestoreLabels(labels))

	// The mgmt-cluster beacon's namespace == cluster name for cluster-scoped mgmt clusters.
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	// Step through each checkpoint in order. The controller transitions between steps on its own
	// once we clear each hook; we just gate the order and inspect intermediate state.
	for _, h := range stepHookKeys {
		cp := WaitForSnapshotRestoreHookPause(t, cs, op, beaconNS, beaconName, h.key, delegateName, h.phase, h.step)
		t.Logf("paused at %s step: phase=%s step=%s delegates=%v", h.name, cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
		// (Inspection point — see godoc above.)
		AdvancePastSnapshotRestoreHook(t, cs, op, beaconNS, beaconName, h.key, delegateName)
	}

	// Final checkpoint: Succeeded phase. By the time we reach this the cluster has fully restored,
	// but the beacon is still held pending the delegate.
	cp := WaitForSnapshotRestoreHookPause(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName, opv1alpha1.OperationPhaseSucceeded, "")
	t.Logf("paused at Succeeded phase: delegates=%v", cp.Beacon.Status.Delegates)
	AdvancePastSnapshotRestoreHook(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName)

	final := WaitForSnapshotRestoreSucceeded(t, cs, op, beaconNS, beaconName)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, final.Status.Phase)

	// Proof-of-restore: the configmap we created pre-snapshot and deleted pre-restore must be back.
	// Validates that the hook framework was transparent to the actual restore work.
	cm.assertRestored(t, fx)
}
