package encryptionkeyrotation

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func preflightStatus() opv1alpha1.EncryptionKeyRotationStatus {
	status := opv1alpha1.EncryptionKeyRotationStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.EncryptionKeyRotationStepPreflight)
	return status
}

func controlPlaneLeader() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cp-1", Namespace: "fleet-default",
			Labels: map[string]string{capr.ClusterNameLabel: "test", capr.ControlPlaneRoleLabel: "true"},
		},
		Type: plan.SecretTypeMachinePlan,
	}
}

// A rotation starts in Preflight, so that nothing on the cluster is changed until it can proceed.
func TestHandlePending_StartsInPreflight(t *testing.T) {
	op := newOp()
	beacon := newBeacon("", false)
	h := &handler{beacons: &fakeBeaconClient{beacon: beacon}}
	s := newScope(op, beacon, &stubAdapter{waitForRegisterOK: true})

	got, err := h.handlePending(s, opv1alpha1.EncryptionKeyRotationStatus{})
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepPreflight, got.Step)
}

// lifecycleLabels returns labels with the lifecycle labels a machine-plan secret of op's cluster
// carries added to them.
func lifecycleLabels(op *opv1alpha1.EncryptionKeyRotation, labels map[string]string) map[string]string {
	labels[planv1alpha1.ClusterLifecycleGroupLabel] = ekrClusterGVK.Group
	labels[planv1alpha1.ClusterLifecycleKindLabel] = op.Spec.ClusterRef.Kind
	labels[planv1alpha1.ClusterLifecycleNameLabel] = op.Spec.ClusterRef.Name
	labels[planv1alpha1.MachineLifecycleGroupLabel] = "cluster.x-k8s.io"
	labels[planv1alpha1.MachineLifecycleKindLabel] = "Machine"
	labels[planv1alpha1.MachineLifecycleNameLabel] = "machine-1"
	return labels
}

// labeledLeader is the control-plane leader as a machine-plan secret of op's cluster.
func labeledLeader(op *opv1alpha1.EncryptionKeyRotation) *corev1.Secret {
	leader := controlPlaneLeader()
	leader.Labels = lifecycleLabels(op, leader.Labels)
	return leader
}

// Passing Preflight pauses the cluster, which is the rotation's point of no return, and moves it on to
// Rotate in the same reconcile. Nothing is dispatched yet.
func TestReconcilePreflight_PausesAndMovesToRotate(t *testing.T) {
	op := newOnChangeOp()
	leader := labeledLeader(op)
	adapter := &stubAdapter{leader: leader}
	secrets := &fakePlanSecrets{items: []*corev1.Secret{leader}}
	h := &handler{secrets: secrets, store: plan.NewStore(secrets)}

	got, err := h.reconcilePreflight(newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter), preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepRotate, got.Step)
	assert.Equal(t, []bool{true}, adapter.pauseCalls, "the cluster is paused on leaving Preflight")
	assert.Equal(t, []ops.WhitelistChange{ops.WhitelistRestores}, adapter.whitelistCalls,
		"pausing is the point of no return, so only a restore may run on the cluster from here")
	assert.Empty(t, secrets.updates, "no plan is assigned in Preflight")
}

// Without a control-plane leader to run rotate-keys on, the rotation waits in Preflight, where it has
// not changed anything yet, rather than pausing the cluster for a rotation that cannot start.
func TestReconcilePreflight_WaitsForALeaderUnpaused(t *testing.T) {
	op := newOnChangeOp()
	adapter := &stubAdapter{}
	h := &handler{secrets: &fakePlanSecrets{}}

	got, err := h.reconcilePreflight(newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter), preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepPreflight, got.Step)
	assert.Equal(t, opv1alpha1.WaitingForSuitableLeaderReason, opv1alpha1.InProgressCondition.GetReason(&got))
	assert.Empty(t, adapter.pauseCalls)
}

// A machine-plan secret whose lifecycle labels don't tie it to the operation's cluster can't be fenced
// by the webhook, so Preflight turns the request away before anything is changed, even before a leader
// is elected.
func TestReconcilePreflight_RejectsMislabeledSecrets(t *testing.T) {
	op := newOnChangeOp()
	leader := labeledLeader(op)
	leader.Labels[planv1alpha1.ClusterLifecycleNameLabel] = "other"
	adapter := &stubAdapter{leader: leader}
	h := &handler{secrets: &fakePlanSecrets{items: []*corev1.Secret{leader}}}

	got, err := h.reconcilePreflight(newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter), preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.RejectedCondition.GetReason(&got))
	assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "fleet-default/cp-1")
	assert.Empty(t, adapter.pauseCalls, "a rejected rotation leaves the cluster unpaused")
}

// A delegate on the Preflight step hook sees the cluster as it was before the rotation: the hook comes
// before the pause.
func TestReconcilePreflight_HookComesBeforeThePause(t *testing.T) {
	op := newOnChangeOp()
	op.Labels = map[string]string{PreflightStepHookLabelPrefix + "inspect": "delegate-a"}
	beacon := newBeacon(ops.BeaconOwnerKey(OperationKind, op), true)
	adapter := &stubAdapter{leader: controlPlaneLeader()}
	h := &handler{beacons: &fakeBeaconClient{beacon: beacon}}
	s := newScope(op, beacon, adapter)

	got, err := h.reconcilePreflight(s, preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepPreflight, got.Step, "the step waits on the delegate")
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.InProgressCondition.GetReason(&got))
	assert.Empty(t, adapter.pauseCalls, "the delegate sees the cluster before it is paused")
	assert.True(t, plan.IsInDelegateChain(s.beacon, "delegate-a"))
}

// Every step has a hook prefix, which handleInProgress uses to tell a delegation from a lost beacon.
func TestStepHookPrefixFor(t *testing.T) {
	assert.Equal(t, PreflightStepHookLabelPrefix, stepHookPrefixFor(opv1alpha1.EncryptionKeyRotationStepPreflight))
	assert.Equal(t, RotateStepHookLabelPrefix, stepHookPrefixFor(opv1alpha1.EncryptionKeyRotationStepRotate))
	assert.Equal(t, RestartStepHookLabelPrefix, stepHookPrefixFor(opv1alpha1.EncryptionKeyRotationStepRestart))
	assert.Empty(t, stepHookPrefixFor(""))
}

// The operation webhook reads a cache, so a rotation can be admitted on a cluster an earlier
// operation has just left requiring a restore. Preflight turns it away before anything is changed, and
// lets it through once the whitelist names it.
func TestReconcilePreflight_HonorsTheWhitelist(t *testing.T) {
	for name, tc := range map[string]struct {
		whitelist    string
		wantRejected bool
	}{
		"a whitelist for restores": {whitelist: opv1alpha1.ETCDSnapshotRestoreResource, wantRejected: true},
		"a whitelist naming it":    {whitelist: opv1alpha1.ETCDSnapshotRestoreResource + "," + opv1alpha1.EncryptionKeyRotationResource},
	} {
		t.Run(name, func(t *testing.T) {
			op := newOnChangeOp()
			leader := labeledLeader(op)
			adapter := &stubAdapter{leader: leader}
			secrets := &fakePlanSecrets{items: []*corev1.Secret{leader}}
			h := &handler{secrets: secrets, store: plan.NewStore(secrets)}
			s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)
			s.clusterAnnotations = map[string]string{opv1alpha1.WhitelistedAnnotation: tc.whitelist}

			got, err := h.reconcilePreflight(s, preflightStatus())
			require.NoError(t, err)
			if !tc.wantRejected {
				assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepRotate, got.Step)
				return
			}
			assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
			assert.Equal(t, opv1alpha1.NotWhitelistedReason, opv1alpha1.RejectedCondition.GetReason(&got))
			assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "only permits "+opv1alpha1.ETCDSnapshotRestoreResource)
			assert.Empty(t, adapter.pauseCalls, "nothing on the cluster is changed")
			assert.Empty(t, secrets.updates)
		})
	}
}
