package encryptionkeyrotation

import (
	"fmt"
	"slices"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each terminal phase leaves the cluster as its phase, and the step the operation stopped in,
// require: Failed and Canceled past the point of no return re-pause and whitelist restores (and say so
// on Finalized), and nothing else touches the cluster. In particular Rejected doesn't: an operation is
// only rejected before it pauses the cluster, so any pause the cluster has is someone else's. A rotation passes its point of no return on leaving Preflight.
//
// Terminal handling runs again on every reconcile until the operation is collected, and by then the
// cluster is no longer the operation's to change, so a second pass writes nothing.
func TestTerminalPhases_SettleTheCluster(t *testing.T) {
	t.Parallel()

	handlers := map[opv1alpha1.OperationPhase]func(*handler, *scope, opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error){
		opv1alpha1.OperationPhaseRejected:  (*handler).handleRejected,
		opv1alpha1.OperationPhaseCanceled:  (*handler).handleCanceled,
		opv1alpha1.OperationPhaseFailed:    (*handler).handleFailed,
		opv1alpha1.OperationPhaseSucceeded: (*handler).handleSucceeded,
	}
	before := []opv1alpha1.EncryptionKeyRotationStep{"", opv1alpha1.EncryptionKeyRotationStepPreflight}
	after := []opv1alpha1.EncryptionKeyRotationStep{opv1alpha1.EncryptionKeyRotationStepRotate, opv1alpha1.EncryptionKeyRotationStepRestart}

	for phase, handle := range handlers {
		for _, step := range slices.Concat(before, after) {
			past := !slices.Contains(before, step)

			t.Run(fmt.Sprintf("%s in step %q", phase, step), func(t *testing.T) {
				t.Parallel()

				adapter := &stubAdapter{}
				h := &handler{beacons: &fakeBeaconClient{}, dynamic: &fakeDynamic{}}
				op := newOp()
				s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)

				status := opv1alpha1.EncryptionKeyRotationStatus{Step: step}
				status.SetPhase(phase)

				got, err := handle(h, s, status)
				require.NoError(t, err)
				require.False(t, got.TerminatedAt.IsZero())

				var wantPause []bool
				var wantWhitelist []ops.WhitelistChange
				restoreRequired := false
				switch {
				case past && (phase == opv1alpha1.OperationPhaseFailed || phase == opv1alpha1.OperationPhaseCanceled):
					wantPause, wantWhitelist = []bool{true}, []ops.WhitelistChange{ops.WhitelistRestores}
					restoreRequired = true
				}
				assert.Equal(t, wantPause, adapter.pauseCalls)
				assert.Equal(t, wantWhitelist, adapter.whitelistCalls)
				assert.Equal(t, restoreRequired, opv1alpha1.FinalizedCondition.GetReason(&got) == opv1alpha1.RestoreRequiredReason,
					"only an operation that left the cluster requiring a restore says so")

				adapter.pauseCalls, adapter.whitelistCalls = nil, nil
				_, err = handle(h, s, got)
				require.NoError(t, err)
				assert.Empty(t, adapter.pauseCalls, "a terminated operation leaves the cluster alone")
			})
		}
	}
}
