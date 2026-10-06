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
	guardedUnpause := []pauseCall{{false, WhitelistKeepsPause}}

	tests := []struct {
		name                string
		phase               opv1alpha1.OperationPhase
		pastPointOfNoReturn bool
		terminated          bool
		err                 error

		wantCalls           []pauseCall
		wantErr             bool
		wantRestoreRequired bool
	}{
		{name: "failed past the point of no return re-pauses and whitelists", phase: opv1alpha1.OperationPhaseFailed, pastPointOfNoReturn: true, wantCalls: repause, wantRestoreRequired: true},
		{name: "canceled past the point of no return re-pauses and whitelists", phase: opv1alpha1.OperationPhaseCanceled, pastPointOfNoReturn: true, wantCalls: repause, wantRestoreRequired: true},
		{name: "failed before the point of no return leaves the cluster", phase: opv1alpha1.OperationPhaseFailed},
		{name: "canceled before the point of no return leaves the cluster", phase: opv1alpha1.OperationPhaseCanceled},
		{name: "rejected unpauses unless whitelisted", phase: opv1alpha1.OperationPhaseRejected, wantCalls: guardedUnpause},
		{name: "rejected is judged the same past the point of no return", phase: opv1alpha1.OperationPhaseRejected, pastPointOfNoReturn: true, wantCalls: guardedUnpause},
		{name: "succeeded already settled the cluster", phase: opv1alpha1.OperationPhaseSucceeded, pastPointOfNoReturn: true},
		{name: "a terminated operation no longer settles", phase: opv1alpha1.OperationPhaseCanceled, pastPointOfNoReturn: true, terminated: true},
		{name: "a terminated rejection no longer unpauses", phase: opv1alpha1.OperationPhaseRejected, terminated: true},
		{
			name: "a cluster object that is gone has nothing to settle", phase: opv1alpha1.OperationPhaseFailed, pastPointOfNoReturn: true,
			err: apierrors.NewNotFound(schema.GroupResource{Group: "cluster.x-k8s.io", Resource: "clusters"}, "c"), wantCalls: repause,
		},
		{
			name: "any other error is retried", phase: opv1alpha1.OperationPhaseCanceled, pastPointOfNoReturn: true,
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

			err := SettleCluster(adapter, status, tt.pastPointOfNoReturn)
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
			wantMessage: "Operation canceled; " + restoreRequiredNote + "; lifecycle hooks were abandoned, no beacon remained to delegate them on",
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
