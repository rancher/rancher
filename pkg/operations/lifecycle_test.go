package operations

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StopDispatchedPlans reports which plans a terminal phase is waiting on through the Finalized
// condition, and UpdateStatus, which otherwise owns that condition outright, must leave the report
// standing until the phase terminates.
func TestUpdateStatusKeepsTheWaitForPlansToStop(t *testing.T) {
	op := &metav1.ObjectMeta{Name: "op-1", Namespace: "fleet-default"}
	spec := &opv1alpha1.OperationSpec{}

	status := &opv1alpha1.OperationStatus{}
	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")
	opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.WaitingForPlansToStopReason)
	opv1alpha1.FinalizedCondition.Message(status, "waiting for the agents to stop the plans it dispatched to node-a")

	UpdateStatus(op, spec, status)
	assert.Equal(t, "False", opv1alpha1.FinalizedCondition.GetStatus(status))
	assert.Equal(t, opv1alpha1.WaitingForPlansToStopReason, opv1alpha1.FinalizedCondition.GetReason(status))
	assert.Equal(t, "waiting for the agents to stop the plans it dispatched to node-a", opv1alpha1.FinalizedCondition.GetMessage(status))

	// A lifecycle hook delegate holding the beacon is reported over it: the wait for the plans only
	// starts once the hook is satisfied.
	hooked := &metav1.ObjectMeta{Name: "op-1", Namespace: "fleet-default", Labels: map[string]string{
		opv1alpha1.CanceledPhaseHookLabelPrefix + "hook": "delegate-a",
	}}
	UpdateStatus(hooked, spec, status)
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.FinalizedCondition.GetReason(status))

	// Once the phase terminates, the operation is finalized.
	opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.WaitingForPlansToStopReason)
	status.SetTerminated()
	UpdateStatus(op, spec, status)
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(status))
	assert.Equal(t, opv1alpha1.FinishedReason, opv1alpha1.FinalizedCondition.GetReason(status))
}

// Once an operation has terminated, Finalized reports a hook as abandoned only when the label for
// the terminal phase it ended in is still there: that is the hook terminal handling gives up on when
// no claim on a beacon remains to delegate it on. A step hook's label, or another phase's, is left
// behind by design once its moment has passed, and doesn't make a clean finish look abandoned.
func TestUpdateStatus_ReportsOnlyTheTerminalPhasesHookAsAbandoned(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		labels      map[string]string
		wantReason  string
		wantMessage string
	}{
		"no hook labels": {
			wantReason:  opv1alpha1.FinishedReason,
			wantMessage: "Operation completed successfully",
		},
		"another phase's hook and a step hook": {
			labels: map[string]string{
				opv1alpha1.FailedPhaseHookLabelPrefix + "inspect": "failure-inspector",
				"rotate.step.hook.operation.cattle.io/verify":     "step-delegate",
			},
			wantReason:  opv1alpha1.FinishedReason,
			wantMessage: "Operation completed successfully",
		},
		"the terminal phase's own hook": {
			labels: map[string]string{
				opv1alpha1.SucceededPhaseHookLabelPrefix + "chain": "follow-up",
				"rotate.step.hook.operation.cattle.io/verify":      "step-delegate",
			},
			wantReason:  opv1alpha1.HookAbandonedReason,
			wantMessage: `Operation completed successfully; the Succeeded phase hook owed to "follow-up" was abandoned, no claim on the beacon remained to delegate it on`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			op := &metav1.ObjectMeta{Name: "op", Labels: tc.labels}
			status := &opv1alpha1.OperationStatus{}
			status.MarkSucceeded()
			status.SetTerminated()

			UpdateStatus(op, &opv1alpha1.OperationSpec{}, status)

			assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(status))
			assert.Equal(t, tc.wantReason, opv1alpha1.FinalizedCondition.GetReason(status))
			assert.Equal(t, tc.wantMessage, opv1alpha1.FinalizedCondition.GetMessage(status))
		})
	}
}
