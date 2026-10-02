package etcdsnapshotsave

import (
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

// withPlanState returns a copy of secret holding p as op assigned it, with the outcome the agent
// recorded for it under plan-state.
func withPlanState(secret *corev1.Secret, op *opv1alpha1.ETCDSnapshotSave, p *planapi.Plan, state planapi.PlanState) *corev1.Secret {
	out := withAssignedPlan(secret, op, p, state)
	out.Data["max-failures"] = []byte("1")
	out.Data["failure-threshold"] = []byte("1")

	switch state {
	case planapi.PlanStateSucceeded:
		out.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
		out.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
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
					withPlanState(newPlanSecret("etcd-1"), op, step.plan(op, adapter), tc.state),
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
			secret := withPlanState(newPlanSecret("etcd-1"), op, step.plan(op, adapter), planapi.PlanStateSucceeded)
			secret.Annotations[planapi.PlanProbesPassedAnnotation] = ""
			secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
			h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

			got, err := step.run(h, newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
			require.NoError(t, err)
			assertWaitingOnPlan(t, step, got)
		}
	})
}

// A follow-up operation computes exactly the plans the one before it did. Whatever the previous
// operation's plan came to, the follow-up must hand its own plan to the agent and wait on it, rather
// than inheriting the previous outcome: advancing on a previous success would skip the snapshot
// entirely, and failing on a previous cancellation would make every save after a canceled one fail.
func TestReconcileSteps_FollowUpOperation(t *testing.T) {
	t.Parallel()

	previous := newOp()
	previous.Name, previous.UID = "save-0", "previous-uid"

	for _, step := range stepsUnderTest() {
		for name, left := range map[string]func(*stubAdapter) *corev1.Secret{
			"succeeded": func(a *stubAdapter) *corev1.Secret {
				return withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateSucceeded)
			},
			"failed": func(a *stubAdapter) *corev1.Secret {
				return withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateFailed)
			},
			"canceled": func(a *stubAdapter) *corev1.Secret {
				return withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateCanceled)
			},
			"in-progress": func(a *stubAdapter) *corev1.Secret {
				return withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateInProgress)
			},
			"pending": func(a *stubAdapter) *corev1.Secret {
				return withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStatePending)
			},
			// A plan the CAPR planner assigned, or one assigned before writers were recorded.
			"assigned by another writer": func(a *stubAdapter) *corev1.Secret {
				secret := withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateSucceeded)
				secret.Annotations[planapi.PlanWriterAnnotation] = "planner"
				return secret
			},
			"assigned with no writer": func(a *stubAdapter) *corev1.Secret {
				secret := withPlanState(newPlanSecret("etcd-1"), previous, step.plan(previous, a), planapi.PlanStateSucceeded)
				delete(secret.Annotations, planapi.PlanWriterAnnotation)
				return secret
			},
		} {
			t.Run(string(step.step)+"/after "+name, func(t *testing.T) {
				t.Parallel()

				followUp := newOp()
				adapter := defaultAdapter()
				secret := left(adapter)
				require.Equal(t, mustMarshal(t, step.plan(followUp, adapter)), secret.Data[planapi.PlanDataKey],
					"the follow-up's plan is identical to the previous one's")

				secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
				h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

				got, err := step.run(h, newScope(followUp, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
				require.NoError(t, err)
				assertWaitingOnPlan(t, step, got)

				require.Len(t, secrets.updates, 1, "the follow-up must assign its own plan")
				written := secrets.updates[0]
				assert.True(t, ops.PlanDispatchedBy(written, ops.BeaconOwnerKey(OperationKind, followUp)))
				assert.False(t, ops.PlanDispatchedBy(written, ops.BeaconOwnerKey(OperationKind, previous)))
				assert.Equal(t, string(planapi.PlanStatePending), string(written.Data[planapi.PlanStateKey]))
				assert.NotContains(t, written.Annotations, planapi.PlanCanceledAnnotation,
					"the previous operation's cancellation must not carry over to the follow-up's plan")
			})
		}
	}
}

// The Store only runs a plan afresh when its content or its writer changes, and every step of one
// operation is written by the same writer. A step whose plan on a node matched the plan the step
// before it left there would be reported on that plan's outcome instead of being run. This drives
// the real step reconcilers through a whole save to pin that no two consecutive steps do so.
func TestConsecutiveStepsAssignDistinctPlans(t *testing.T) {
	t.Parallel()

	op := newOp()
	adapter := defaultAdapter()
	secret := newPlanSecret("etcd-1")

	var previous []byte
	for _, step := range stepsUnderTest() {
		secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
		h := &handler{secrets: secrets, store: planapi.NewStore(secrets)}

		got, err := step.run(h, newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: step.step})
		require.NoError(t, err)
		assertWaitingOnPlan(t, step, got)
		require.Len(t, secrets.updates, 1, "step %s must assign a plan of its own", step.step)

		secret = secrets.updates[0]
		assert.NotEqual(t, previous, secret.Data[planapi.PlanDataKey], "step %s assigns the same plan as the step before it", step.step)
		previous = secret.Data[planapi.PlanDataKey]

		// The agent completes it, so the next step runs.
		secret.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateSucceeded)
		secret.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
		secret.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
	}
}

// Both terminal outcomes that leave work behind stop it before the beacon is released, but only the
// work that is still running: a plan that already finished has nothing left to stop.
func TestTerminalPhasesCancelActivePlans(t *testing.T) {
	t.Parallel()

	for phase, handle := range map[opv1alpha1.OperationPhase]func(*handler, *scope, opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error){
		opv1alpha1.OperationPhaseCanceled: (*handler).handleCanceled,
		opv1alpha1.OperationPhaseFailed:   (*handler).handleFailed,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			op := newOp()
			adapter := defaultAdapter()
			s := newScope(op, newBeacon(testOwnerKey, true), adapter)

			var events []string
			secrets := &fakePlanSecrets{events: &events, items: []*corev1.Secret{
				withPlanState(newPlanSecret("running"), op, expectedSavePlan(op, adapter), planapi.PlanStateInProgress),
				withPlanState(newPlanSecret("finished"), op, expectedSavePlan(op, adapter), planapi.PlanStateSucceeded),
			}}
			h := &handler{beacons: &fakeBeaconClient{beacon: s.beacon, events: &events}, secrets: secrets, store: planapi.NewStore(secrets), dynamic: &fakeDynamic{}}

			status := opv1alpha1.ETCDSnapshotSaveStatus{}
			status.SetPhase(phase)

			got, err := handle(h, s, status)
			require.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero())
			assert.Equal(t, []string{"cancel-plan/running", "beacon-write"}, events,
				"only the running plan is canceled, and before the beacon is released")
			require.Len(t, secrets.updates, 1)
			assert.Equal(t, "true", secrets.updates[0].Annotations[planapi.PlanCanceledAnnotation])
			assert.Equal(t, testOwnerKey, secrets.updates[0].Annotations[planapi.PlanWriterAnnotation])
		})

		// An operation that has lost the beacon must not reach into the work of whoever holds it now,
		// and still terminates.
		t.Run(string(phase)+" without the beacon", func(t *testing.T) {
			t.Parallel()

			op := newOp()
			adapter := defaultAdapter()
			s := newScope(op, newBeacon("someone-else", true), adapter)

			secrets := &fakePlanSecrets{items: []*corev1.Secret{
				withPlanState(newPlanSecret("running"), op, expectedSavePlan(op, adapter), planapi.PlanStateInProgress),
			}}
			beacons := &fakeBeaconClient{beacon: s.beacon}
			h := &handler{beacons: beacons, secrets: secrets, store: planapi.NewStore(secrets), dynamic: &fakeDynamic{}}

			status := opv1alpha1.ETCDSnapshotSaveStatus{}
			status.SetPhase(phase)

			got, err := handle(h, s, status)
			require.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero())
			assert.Empty(t, secrets.updates)
			assert.Empty(t, beacons.statusUpdates)
		})
	}
}

// TestFollowUpAfterCanceledOperation walks the whole sequence the tests above cover in pieces: an
// operation is canceled with its plan in flight, the agent records the cancellation, and a follow-up
// of the same shape then runs to completion on the same machine-plan secret.
func TestFollowUpAfterCanceledOperation(t *testing.T) {
	t.Parallel()

	adapter := defaultAdapter()
	canceled := newOp()
	canceled.Name, canceled.UID = "save-0", "canceled-uid"
	canceledKey := ops.BeaconOwnerKey(OperationKind, canceled)
	followUp := newOp()

	// The canceled operation's save plan is in flight when it is called off.
	secret := withPlanState(newPlanSecret("etcd-1"), canceled, expectedSavePlan(canceled, adapter), planapi.PlanStateInProgress)

	secrets := &fakePlanSecrets{items: []*corev1.Secret{secret}}
	h := &handler{
		beacons: &fakeBeaconClient{beacon: newBeacon(canceledKey, true)},
		secrets: secrets,
		store:   planapi.NewStore(secrets),
		dynamic: &fakeDynamic{},
	}
	status := opv1alpha1.ETCDSnapshotSaveStatus{}
	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")
	_, err := h.handleCanceled(newScope(canceled, newBeacon(canceledKey, true), adapter), status)
	require.NoError(t, err)
	require.Len(t, secrets.updates, 1)
	secret = secrets.updates[0]
	require.Equal(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation])

	// The agent observes the annotation and records the plan as canceled.
	secret.Data[planapi.PlanStateKey] = []byte(planapi.PlanStateCanceled)

	// The follow-up assigns its own save plan over it, identical to the canceled one's.
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
	// not reach the follow-up's plan: the follow-up holds the beacon now, and even with a stale view
	// in which the canceled operation still held it, the plan is no longer one it wrote.
	for name, beacon := range map[string]string{"follow-up's beacon": ops.BeaconOwnerKey(OperationKind, followUp), "stale beacon": canceledKey} {
		secrets = &fakePlanSecrets{items: []*corev1.Secret{secret}}
		n, err := ops.CancelDispatchedPlans(planapi.NewStore(secrets), secrets, newScope(canceled, nil, adapter).clusterObj, "fleet-default",
			canceled, canceledKey, newBeacon(beacon, true))
		require.NoError(t, err, name)
		assert.Zero(t, n, name)
		assert.Empty(t, secrets.updates, name)
	}
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

func mustMarshal(t *testing.T, p *planapi.Plan) []byte {
	t.Helper()
	return withAssignedPlan(newPlanSecret("scratch"), newOp(), p, "").Data[planapi.PlanDataKey]
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
