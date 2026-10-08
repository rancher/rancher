package operations

import (
	"errors"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type pauseCall struct {
	pause     bool
	whitelist WhitelistChange
}

// pauseRecorder records the PauseCluster calls made on it. SettleCluster calls nothing else, so the
// rest of the Adapter is left to the nil embedded interface.
type pauseRecorder struct {
	Adapter
	calls []pauseCall
	err   error
}

func (a *pauseRecorder) PauseCluster(pause bool, whitelist WhitelistChange) error {
	a.calls = append(a.calls, pauseCall{pause, whitelist})
	return a.err
}

func TestSettleCluster(t *testing.T) {
	t.Parallel()

	repause := []pauseCall{{true, WhitelistRestores}}

	tests := []struct {
		name            string
		phase           opv1alpha1.OperationPhase
		requiresRestore bool
		terminated      bool
		err             error

		wantCalls           []pauseCall
		wantErr             bool
		wantRestoreRequired bool
	}{
		{name: "failed past the point of no return re-pauses and whitelists", phase: opv1alpha1.OperationPhaseFailed, requiresRestore: true, wantCalls: repause, wantRestoreRequired: true},
		{name: "canceled past the point of no return re-pauses and whitelists", phase: opv1alpha1.OperationPhaseCanceled, requiresRestore: true, wantCalls: repause, wantRestoreRequired: true},
		{name: "failed before the point of no return leaves the cluster", phase: opv1alpha1.OperationPhaseFailed},
		{name: "canceled before the point of no return leaves the cluster", phase: opv1alpha1.OperationPhaseCanceled},
		// A rejected operation never paused the cluster, so whatever pause it has is someone else's: an
		// earlier operation that left it requiring a restore, the operation it conflicted with, or a user.
		{name: "rejected leaves the cluster alone", phase: opv1alpha1.OperationPhaseRejected},
		{name: "rejected leaves the cluster alone whatever its step", phase: opv1alpha1.OperationPhaseRejected, requiresRestore: true},
		{name: "succeeded already settled the cluster", phase: opv1alpha1.OperationPhaseSucceeded, requiresRestore: true},
		{name: "a terminated operation no longer settles", phase: opv1alpha1.OperationPhaseCanceled, requiresRestore: true, terminated: true},
		{
			name: "a cluster object that is gone has nothing to settle", phase: opv1alpha1.OperationPhaseFailed, requiresRestore: true,
			err: apierrors.NewNotFound(schema.GroupResource{Group: "cluster.x-k8s.io", Resource: "clusters"}, "c"), wantCalls: repause,
		},
		{
			name: "any other error is retried", phase: opv1alpha1.OperationPhaseCanceled, requiresRestore: true,
			err: errors.New("apiserver is down"), wantCalls: repause, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter := &pauseRecorder{err: tt.err}
			status := &opv1alpha1.OperationStatus{}
			status.SetPhase(tt.phase)
			if tt.terminated {
				status.SetTerminated()
			}

			err := SettleCluster(adapter, status, tt.requiresRestore)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCalls, adapter.calls)

			if tt.wantRestoreRequired {
				assert.Equal(t, opv1alpha1.RestoreRequiredReason, opv1alpha1.FinalizedCondition.GetReason(status))
				assert.Contains(t, opv1alpha1.FinalizedCondition.GetMessage(status), restoreRequiredNote)
			} else {
				assert.NotEqual(t, opv1alpha1.RestoreRequiredReason, opv1alpha1.FinalizedCondition.GetReason(status),
					"only an operation that left the cluster whitelisted says so")
			}
		})
	}
}

// The note is recorded as the operation terminates, and nothing afterwards would know to record it
// again, so UpdateStatus keeps it on Finalized rather than replacing it with FinishedReason.
func TestUpdateStatus_KeepsRestoreRequired(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		labels      map[string]string
		wantMessage string
	}{
		"without hooks": {
			wantMessage: "Operation canceled; " + restoreRequiredNote,
		},
		"with an abandoned hook": {
			labels:      map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "verify": "delegate-a"},
			wantMessage: "Operation canceled; " + restoreRequiredNote + `; the Canceled phase hook owed to "delegate-a" was abandoned, no claim on the beacon remained to delegate it on`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			op := &metav1.ObjectMeta{Name: "op", Labels: tc.labels}
			status := &opv1alpha1.OperationStatus{}
			status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")
			MarkRestoreRequired(status)
			status.SetTerminated()

			for range 2 {
				UpdateStatus(op, &opv1alpha1.OperationSpec{}, status)

				assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(status))
				assert.Equal(t, opv1alpha1.RestoreRequiredReason, opv1alpha1.FinalizedCondition.GetReason(status))
				assert.Equal(t, tc.wantMessage, opv1alpha1.FinalizedCondition.GetMessage(status))
				assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(status),
					"the outcome condition keeps the reason the operation ended with")
			}
		})
	}
}
