package imported

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotsave"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
)

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSave brings up an imported single-node cluster and drives
// a plain ETCDSnapshotSave operation through the operation.cattle.io/v1alpha1 controller to the
// Succeeded phase. This is the baseline path — no lifecycle hooks attached — so the controller
// acquires the beacon, runs Save+Restart, and releases the beacon on its own.
func Test_Imported_Operation_SetD_ImportedETCDSnapshotSave(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-snapshot", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
}

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveLifecycleHook brings up an imported single-node
// cluster and walks an ETCDSnapshotSave through every state-machine checkpoint by attaching a
// lifecycle-hook label for each step + the Succeeded phase. At each checkpoint the test:
//
//  1. Waits for the operation to land in the expected (phase, step) AND for the controller to push
//     the named delegate onto the beacon — proving the hook actually fired and the controller
//     observed the label.
//  2. (Hook-of-the-future) Inspects intermediate state. For ETCDSnapshotSave the steps are
//     mechanical (assign save plan → assign restart plan), so the only assertion we make is
//     "reached this checkpoint". For richer operations — ETCDSnapshotRestore (Shutdown / Restore /
//     PodCleanup / Restart / NodeCleanup) and EncryptionKeyRotation — this is where a test would
//     read plan secrets, validate their instructions, or even override the system-agent plan to
//     simulate a partial failure.
//  3. Clears the hook label and pops the delegate, releasing the controller to do the actual
//     step work before reaching the next gated checkpoint.
//
// This is the canonical pattern for any future hook-driven operation test in this package: the
// step-level pauses give the test deterministic interleavings with the controller without having
// to race on watch events.
func Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveLifecycleHook(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-snapshot-lifecycle-hook", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	// hookName is the suffix appended after the controller's well-known prefix — the controller
	// doesn't interpret it, it just trims the prefix off and reads the label value as the delegate.
	// Reusing one name + one delegate keeps the assertions simple; in a real multi-stakeholder
	// hook scenario each hooking controller would pick a unique suffix.
	const (
		hookName     = "v2prov-e2e-test"
		delegateName = "v2prov-e2e-test-delegate"
	)
	preflightHookKey := etcdsnapshotsave.PreflightStepHookLabelPrefix + hookName
	saveHookKey := etcdsnapshotsave.SaveStepHookLabelPrefix + hookName
	restartHookKey := etcdsnapshotsave.RestartStepHookLabelPrefix + hookName
	succeededHookKey := planv1alpha1.SucceededPhaseHookLabelPrefix + hookName

	// Attach all three hook labels up front. Each prefix is scoped to a specific
	// phase/step handler, so the controller only checks the relevant key when it enters that
	// handler — they don't interfere with each other.
	op := CreateETCDSnapshotSaveOp(t, cs, fx.ns.Name, fx.clusterRef, WithSaveLabels(map[string]string{
		preflightHookKey: delegateName,
		saveHookKey:      delegateName,
		restartHookKey:   delegateName,
		succeededHookKey: delegateName,
	}))

	// The mgmt-cluster beacon lives in the namespace named after the cluster — cluster-scoped
	// mgmt clusters use namespace == name.
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	// Op should be InProgress at the preflight step. The controller's reconcileSave runs the hook
	// check before assigning any plans, so no etcd-snapshot save has been issued downstream yet.
	cp := WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, preflightHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepPreflight)
	t.Logf("paused at Preflight step: phase=%s step=%s delegates=%v", cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
	// (Inspection point — see godoc above.)
	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, preflightHookKey, delegateName)

	// Op should be InProgress at the Save step. The controller's reconcileSave runs the hook
	// check before assigning any plans, so no etcd-snapshot save has been issued downstream yet.
	cp = WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, saveHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepSave)
	t.Logf("paused at Save step: phase=%s step=%s delegates=%v", cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
	// (Inspection point — see godoc above.)
	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, saveHookKey, delegateName)

	// The save plan has now been issued and applied (otherwise the controller would not have
	// transitioned to Restart). reconcileRestart runs its hook check before assigning the
	// systemctl-restart plan.
	cp = WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepRestart)
	t.Logf("paused at Restart step: phase=%s step=%s delegates=%v", cp.Op.Status.Phase, cp.Op.Status.Step, cp.Beacon.Status.Delegates)
	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName)

	// Phase transitioned to Succeeded but the hook gates beacon release. handleSucceeded's hook
	// check runs before the beacon ToggleBeacon/ReleaseBeacon pair, so the beacon's owner field is
	// still set to the controller and our delegate is still on the chain.
	cp = WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName,
		opv1alpha1.OperationPhaseSucceeded, "")
	t.Logf("paused at Succeeded phase: phase=%s delegates=%v", cp.Op.Status.Phase, cp.Beacon.Status.Delegates)
	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, succeededHookKey, delegateName)

	// All hooks cleared — the controller's next reconcile should drain the delegate chain (it's
	// already drained by AdvancePastSnapshotSaveHook) and release the beacon.
	final := WaitForSnapshotSaveSucceeded(t, cs, op, beaconNS, beaconName)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, final.Status.Phase)
}
