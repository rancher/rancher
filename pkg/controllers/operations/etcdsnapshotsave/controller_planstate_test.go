package etcdsnapshotsave

import (
	"encoding/json"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// stepUnderTest is one of the plan-dispatching steps, along with the plan it assigns and the step
// that follows it once every plan has been applied.
type stepUnderTest struct {
	step     opv1alpha1.ETCDSnapshotSaveStep
	plan     func(*opv1alpha1.ETCDSnapshotSave, *stubAdapter) *planapi.Plan
	run      func(*handler, *scope, opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error)
	advanced func(t *testing.T, got opv1alpha1.ETCDSnapshotSaveStatus)
}

func stepsUnderTest() []stepUnderTest {
	return []stepUnderTest{
		{
			step: opv1alpha1.ETCDSnapshotSaveStepSave,
			plan: expectedSavePlan,
			run:  (*handler).reconcileSave,
			advanced: func(t *testing.T, got opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepRestart, got.Step)
				assert.Empty(t, string(got.Phase))
			},
		},
		{
			step: opv1alpha1.ETCDSnapshotSaveStepRestart,
			plan: expectedRestartPlan,
			run:  (*handler).reconcileRestart,
			advanced: func(t *testing.T, got opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, got.Phase)
			},
		},
	}
}

// withPlanState returns a copy of secret holding p, with the outcome an agent which supports
// plan-state recorded for it. Unlike withAppliedPlan, nothing here is read by the checksum flow: a
// plan-state of succeeded is the only thing saying the plan was applied.
func withPlanState(t *testing.T, secret *corev1.Secret, p *planapi.Plan, state planapi.PlanState) *corev1.Secret {
	t.Helper()

	data, err := json.Marshal(p)
	require.NoError(t, err)

	out := secret.DeepCopy()
	if out.Data == nil {
		out.Data = map[string][]byte{}
	}
	out.Data[planapi.PlanDataKey] = data
	out.Data[planapi.PlanStateKey] = []byte(state)
	out.Data["max-failures"] = []byte("1")
	out.Data["failure-threshold"] = []byte("1")

	switch state {
	case planapi.PlanStateSucceeded:
		out.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
		out.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
	case planapi.PlanStateFailed:
		out.Data["failed-checksum"] = []byte(planapi.PlanHash(data))
		out.Data["failure-count"] = []byte("1")
	case planapi.PlanStateCanceled:
		out.Annotations[planapi.PlanCanceledAnnotation] = "true"
	}
	return out
}

// Each step acts on the outcome the agent recorded under plan-state for the plan it assigned, without
// reassigning it.
func TestReconcileSteps_PlanState(t *testing.T) {
	t.Parallel()

	for _, step := range stepsUnderTest() {
		for _, tc := range []struct {
			state  planapi.PlanState
			assert func(t *testing.T, step stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus)
		}{
			{state: planapi.PlanStateSucceeded, assert: func(t *testing.T, step stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus) {
				step.advanced(t, got)
			}},
			{state: planapi.PlanStatePending, assert: assertWaitingOnPlan},
			{state: planapi.PlanStateInProgress, assert: assertWaitingOnPlan},
			{state: planapi.PlanStatePaused, assert: assertWaitingOnPlan},
			{state: planapi.PlanStateFailed, assert: func(t *testing.T, step stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus) {
				assertPlanFailed(t, step, got)
				assert.NotContains(t, opv1alpha1.FailedCondition.GetMessage(&got), "canceled")
			}},
			// A plan canceled out from under a running operation will never complete, so the
			// operation cannot wait on it. It still fails as a plan failure, but says why.
			{state: planapi.PlanStateCanceled, assert: func(t *testing.T, step stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus) {
				assertPlanFailed(t, step, got)
				assert.Contains(t, opv1alpha1.FailedCondition.GetMessage(&got), "fleet-default/etcd-1: the in-progress plan was canceled externally")
			}},
		} {
			t.Run(string(step.step)+"/"+string(tc.state), func(t *testing.T) {
				t.Parallel()

				op := newOp()
				adapter := defaultAdapter()
				secrets := &fakePlanSecrets{items: []*corev1.Secret{
					withPlanState(t, newPlanSecret("etcd-1"), step.plan(op, adapter), tc.state),
				}}
				h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

				got, err := step.run(h, newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
				require.NoError(t, err)
				tc.assert(t, step, got)
				assert.Empty(t, secrets.updates, "the plan this operation already assigned must not be rewritten")
			})
		}
	}

	t.Run("succeeded but probes have not passed", func(t *testing.T) {
		t.Parallel()

		for _, step := range stepsUnderTest() {
			op := newOp()
			adapter := defaultAdapter()
			secret := withPlanState(t, newPlanSecret("etcd-1"), step.plan(op, adapter), planapi.PlanStateSucceeded)
			secret.Annotations[planapi.PlanProbesPassedAnnotation] = ""
			secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
			h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

			got, err := step.run(h, newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
			require.NoError(t, err)
			assertWaitingOnPlan(t, step, got)
		}
	})
}

// A follow-up operation computes the same plans as the one before it, apart from the operation
// environment. Whatever the previous operation's plan came to, the follow-up must hand its own plan
// to the agent and wait on it, rather than inheriting the previous outcome: advancing on a previous
// success would skip the snapshot entirely, and failing on a previous cancellation would make every
// save after a canceled one fail.
func TestReconcileSteps_FollowUpOperation(t *testing.T) {
	t.Parallel()

	for _, step := range stepsUnderTest() {
		for _, previousState := range []planapi.PlanState{
			planapi.PlanStateSucceeded,
			planapi.PlanStateFailed,
			planapi.PlanStateCanceled,
			planapi.PlanStateInProgress,
			planapi.PlanStatePending,
			"", // left by an agent which predates plan-state
		} {
			t.Run(string(step.step)+"/after "+string(previousState), func(t *testing.T) {
				t.Parallel()

				previous := newOp()
				previous.Name, previous.UID = "save-0", "previous-uid"
				followUp := newOp()
				adapter := defaultAdapter()

				left := withPlanState(t, newPlanSecret("etcd-1"), step.plan(previous, adapter), previousState)
				if previousState == planapi.PlanStateSucceeded || previousState == "" {
					// Also what the checksum flow would read as applied.
					left.Data["appliedPlan"] = left.Data[planapi.PlanDataKey]
				}
				secrets := &fakePlanSecrets{items: []*corev1.Secret{left}}
				h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

				got, err := step.run(h, newScope(followUp, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
				require.NoError(t, err)
				assertWaitingOnPlan(t, step, got)

				require.Len(t, secrets.updates, 1, "the follow-up must assign its own plan")
				written := secrets.updates[0]
				assert.True(t, ops.PlanDispatchedBy(written, followUp))
				assert.False(t, ops.PlanDispatchedBy(written, previous))
				assert.Equal(t, string(planapi.PlanStatePending), string(written.Data[planapi.PlanStateKey]))
				assert.NotContains(t, written.Annotations, planapi.PlanCanceledAnnotation,
					"the previous operation's cancellation must not carry over to the follow-up's plan")
			})
		}
	}
}

// TestFollowUpAfterCanceledOperation walks the whole sequence the two tests above cover in pieces:
// an operation is canceled with its plan in flight, the agent records the cancellation, and a
// follow-up of the same shape then runs to completion on the same machine-plan secret.
func TestFollowUpAfterCanceledOperation(t *testing.T) {
	t.Parallel()

	adapter := defaultAdapter()
	canceled := newOp()
	canceled.Name, canceled.UID = "save-0", "canceled-uid"
	followUp := newOp()

	// The canceled operation's save plan is in flight when it is called off.
	secret := withPlanState(t, newPlanSecret("etcd-1"), expectedSavePlan(canceled, adapter), planapi.PlanStateInProgress)

	secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
	h := &handler{
		beacons: &fakeBeaconClient{beacon: newBeacon(ops.BeaconOwnerKey(OperationKind, canceled), true)},
		secrets: secrets,
		store:   planapi.NewStore(secrets),
		dynamic: &fakeDynamic{},
	}
	status := opv1alpha1.ETCDSnapshotSaveStatus{}
	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")
	_, err := h.handleCanceled(newScope(canceled, newBeacon(ops.BeaconOwnerKey(OperationKind, canceled), true), adapter), status)
	require.NoError(t, err)
	require.Len(t, secrets.updates, 1)
	secret = secrets.updates[0]
	require.Equal(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation])

	// The agent observes the annotation and records the plan as canceled.
	secret.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateCanceled)

	// The follow-up assigns its own save plan over it.
	secret = reconcileStepOnce(t, secret, followUp, adapter, stepsUnderTest()[0], assertWaitingOnPlan)

	// The agent runs it, and the follow-up moves on to restart.
	secret.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateSucceeded)
	secret.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
	secret.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
	secrets = &fakePlanSecrets{items: []*corev1.Secret{secret}}
	h = &handler{secrets: secrets, store: planapi.NewStore(secrets)}
	got, err := h.reconcileSave(newScope(followUp, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepRestart, got.Step)
	assert.Empty(t, secrets.updates)

	// The canceled operation canceling its plans again — its terminal handling retried, say — must
	// not reach the follow-up's plan.
	secrets = &fakePlanSecrets{items: []*corev1.Secret{secret}}
	n, err := ops.CancelDispatchedPlans(planapi.NewStore(secrets), secrets, newScope(canceled, nil, adapter).clusterObj, "fleet-default", canceled)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, secrets.updates)
}

// reconcileStepOnce runs step for op over secret, asserts on the outcome, and returns the secret as
// the step left it.
func reconcileStepOnce(t *testing.T, secret *corev1.Secret, op *opv1alpha1.ETCDSnapshotSave, adapter *stubAdapter, step stepUnderTest,
	assertion func(*testing.T, stepUnderTest, opv1alpha1.ETCDSnapshotSaveStatus),
) *corev1.Secret {
	t.Helper()

	secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
	h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}
	got, err := step.run(h, newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
	require.NoError(t, err)
	assertion(t, step, got)
	require.Len(t, secrets.updates, 1)
	written := secrets.updates[0]
	assert.Equal(t, string(planapi.PlanStatePending), string(written.Data[planapi.PlanStateKey]))
	assert.NotContains(t, written.Annotations, planapi.PlanCanceledAnnotation)
	return written
}

func assertWaitingOnPlan(t *testing.T, step stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus) {
	t.Helper()

	assert.Empty(t, string(got.Phase), "phase must not change while waiting on a plan")
	assert.Equal(t, step.step, got.Step, "step must not advance while waiting on a plan")
	assert.Equal(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&got))
}

func assertPlanFailed(t *testing.T, _ stepUnderTest, got opv1alpha1.ETCDSnapshotSaveStatus) {
	t.Helper()

	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PlanFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
}
