package encryptionkeyrotation

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
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

// Passing Preflight pauses the cluster, which is the rotation's point of no return, and moves it on to
// Rotate in the same reconcile. Nothing is dispatched yet.
func TestReconcilePreflight_PausesAndMovesToRotate(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{leader: controlPlaneLeader()}
	secrets := &fakePlanSecrets{}
	h := &handler{secrets: secrets, store: plan.NewStore(secrets)}

	got, err := h.reconcilePreflight(newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter), preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepRotate, got.Step)
	assert.Equal(t, []bool{true}, adapter.pauseCalls, "the cluster is paused on leaving Preflight")
	assert.Empty(t, secrets.updates, "no plan is assigned in Preflight")
}

// Without a control-plane leader to run rotate-keys on, the rotation waits in Preflight, where it has
// not changed anything yet, rather than pausing the cluster for a rotation that cannot start.
func TestReconcilePreflight_WaitsForALeaderUnpaused(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{}
	h := &handler{}

	got, err := h.reconcilePreflight(newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter), preflightStatus())
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.EncryptionKeyRotationStepPreflight, got.Step)
	assert.Equal(t, opv1alpha1.WaitingForSuitableLeaderReason, opv1alpha1.InProgressCondition.GetReason(&got))
	assert.Empty(t, adapter.pauseCalls)
}

// A delegate on the Preflight step hook sees the cluster as it was before the rotation: the hook comes
// before the pause.
func TestReconcilePreflight_HookComesBeforeThePause(t *testing.T) {
	op := newOp()
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
