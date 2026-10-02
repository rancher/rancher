package imported

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotsave"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// These tests depend on a system-agent which supports plan-state (and with it, plan pause and
// cancellation). They are not part of any CI scope.

const (
	cancelHookName     = "v2prov-e2e-cancel"
	cancelDelegateName = "v2prov-e2e-cancel-delegate"
)

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveCancel cancels an ETCDSnapshotSave with its
// save plan still unfinished, and asserts that the cancellation reaches the machine-plan secret rather
// than stopping at the operation's status: the plan the operation dispatched is annotated as
// canceled, and the agent records plan-state canceled for it.
//
// An etcd snapshot is too quick to cancel reliably while it runs, so the test holds the plan instead:
// while the operation waits on its Save step hook, it pauses the node's machine-plan, so the agent
// holds the save plan the operation then assigns rather than running it. A paused plan is still
// unfinished, which is what the operation cancels on its way out; a plan that already succeeded
// would be left alone.
func Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveCancel(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-snapshot-cancel", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	cancelSnapshotSaveWhilePaused(t, cs, fx)
}

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveFollowUp runs saves of the same shape one
// after another. Each computes exactly the plans the one before it did, and each must still be run
// by the agent on its own account rather than inheriting the outcome of the plan the one before it
// left on the machine-plan secret:
//
//   - after a canceled save, the follow-up must not be reported as canceled (or failed) on the
//     strength of the previous plan's plan-state, and must clear the cancellation from the secret;
//   - after a succeeded save, the follow-up must not be reported as already applied without the agent
//     having run anything.
//
// plan-revision is what tells the two apart from the outside: the agent increments it each time it
// picks a pending plan up for execution, so a save that really ran both of its steps advances it by
// at least two.
func Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveFollowUp(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-snapshot-follow-up", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	canceled := cancelSnapshotSaveWhilePaused(t, cs, fx)
	canceledPlan := canceled.Data[planapi.PlanDataKey]
	revision := planRevision(t, canceled)

	// After the canceled save. Its machine-plan is still paused, so the follow-up lifts the pause
	// while it holds the beacon at its Save step hook, before it assigns its own save plan.
	restartHookKey := etcdsnapshotsave.RestartStepHookLabelPrefix + cancelHookName
	saveHookKey := etcdsnapshotsave.SaveStepHookLabelPrefix + cancelHookName
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	followUp := CreateETCDSnapshotSaveOp(t, cs, fx.ns.Name, fx.clusterRef, WithSaveLabels(map[string]string{
		saveHookKey:    cancelDelegateName,
		restartHookKey: cancelDelegateName,
	}))
	followUpKey := ops.BeaconOwnerKey(etcdsnapshotsave.OperationKind, followUp)

	WaitForSnapshotSaveHookPause(t, cs, followUp, beaconNS, beaconName, saveHookKey, cancelDelegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepSave)
	setPlanPaused(t, cs, fx, false)
	AdvancePastSnapshotSaveHook(t, cs, followUp, beaconNS, beaconName, saveHookKey, cancelDelegateName)

	// At the Restart step hook the follow-up's save plan has run: the very plan the canceled save
	// left on the secret, now assigned by the follow-up and run to completion.
	WaitForSnapshotSaveHookPause(t, cs, followUp, beaconNS, beaconName, restartHookKey, cancelDelegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepRestart)
	secret := etcdPlanSecret(t, cs, fx)
	assert.Equal(t, string(canceledPlan), string(secret.Data[planapi.PlanDataKey]), "the follow-up's save plan is identical to the canceled one's")
	assert.True(t, ops.PlanDispatchedBy(secret, followUpKey), "the plan on the secret should be the follow-up's own")
	assert.Equal(t, string(planapi.PlanStateSucceeded), string(secret.Data[planapi.PlanStateKey]))
	assert.NotEqual(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation],
		"assigning the plan afresh must clear the previous plan's cancellation")
	assert.Greater(t, planRevision(t, secret), revision, "the agent should have picked up the follow-up's save plan")
	AdvancePastSnapshotSaveHook(t, cs, followUp, beaconNS, beaconName, restartHookKey, cancelDelegateName)

	WaitForSnapshotSaveSucceeded(t, cs, followUp, beaconNS, beaconName)
	revision = assertFollowUpRan(t, cs, fx, followUpKey, revision, "after a canceled save")

	// After a succeeded save.
	op := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	assertFollowUpRan(t, cs, fx, ops.BeaconOwnerKey(etcdsnapshotsave.OperationKind, op), revision, "after a succeeded save")
}

// assertFollowUpRan asserts that the save whose beacon key is writer ran both of its steps on the
// node, starting from the given plan-revision, and returns the revision it left.
func assertFollowUpRan(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, writer string, revision int, leg string) int {
	t.Helper()

	secret := etcdPlanSecret(t, cs, fx)
	assert.True(t, ops.PlanDispatchedBy(secret, writer), "%s: the plan on the secret should be the follow-up's own", leg)
	assert.Equal(t, "restart", firstInstructionName(t, secret), "%s: the follow-up's restart plan should be the last it assigned", leg)
	assert.Equal(t, string(planapi.PlanStateSucceeded), string(secret.Data[planapi.PlanStateKey]), leg)
	assert.NotEqual(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation], leg)

	next := planRevision(t, secret)
	assert.GreaterOrEqual(t, next, revision+2,
		"%s: the agent should have picked up both the save and the restart plan of the follow-up", leg)
	return next
}

// cancelSnapshotSaveWhilePaused runs an ETCDSnapshotSave whose save plan the agent holds paused,
// cancels it, and waits for both the operation and the plan it dispatched to report the
// cancellation. Returns the machine-plan secret as it stands once the agent has recorded the plan as
// canceled; the secret is left paused.
func cancelSnapshotSaveWhilePaused(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) *corev1.Secret {
	t.Helper()

	saveHookKey := etcdsnapshotsave.SaveStepHookLabelPrefix + cancelHookName
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	op := CreateETCDSnapshotSaveOp(t, cs, fx.ns.Name, fx.clusterRef, WithSaveLabels(map[string]string{
		saveHookKey: cancelDelegateName,
	}))
	ownerKey := ops.BeaconOwnerKey(etcdsnapshotsave.OperationKind, op)

	// Pause the node's machine-plan while the operation waits on its Save step hook, before it has
	// assigned anything, so the agent holds the save plan instead of running it.
	WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, saveHookKey, cancelDelegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepSave)
	setPlanPaused(t, cs, fx, true)
	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, saveHookKey, cancelDelegateName)

	secret := waitForPlanSecret(t, cs, fx, "the agent to hold the save plan paused", func(secret *corev1.Secret) bool {
		return ops.PlanDispatchedBy(secret, ownerKey) && planapi.PlanState(secret.Data[planapi.PlanStateKey]) == planapi.PlanStatePaused
	})
	assert.Equal(t, "snapshot", firstInstructionName(t, secret))

	latest, err := cs.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, opv1alpha1.OperationPhaseInProgress, latest.Status.Phase, "the operation should be waiting on its paused plan")
	require.Equal(t, opv1alpha1.ETCDSnapshotSaveStepSave, latest.Status.Step)

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := cs.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		latest = latest.DeepCopy()
		latest.Spec.Cancel = true
		_, err = cs.Operation.ETCDSnapshotSave().Update(latest)
		return err
	})
	require.NoError(t, err, "cancel op %s/%s", op.Namespace, op.Name)

	err = utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := cs.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch got.Status.Phase {
		case opv1alpha1.OperationPhaseSucceeded, opv1alpha1.OperationPhaseFailed, opv1alpha1.OperationPhaseRejected:
			return false, fmt.Errorf("operation reached %s instead of Canceled at step %q", got.Status.Phase, got.Status.Step)
		}
		latest = got
		// Terminated is recorded only after the dispatched plans were canceled and the beacon released.
		return got.Status.Phase == opv1alpha1.OperationPhaseCanceled && !got.Status.TerminatedAt.IsZero(), nil
	})
	if err != nil {
		handleError(t, cs, fx.mgmtCluster.Name, err)
	}
	assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(latest))

	secret = waitForPlanSecret(t, cs, fx, "the agent to record the plan as canceled", func(secret *corev1.Secret) bool {
		return planapi.PlanState(secret.Data[planapi.PlanStateKey]) == planapi.PlanStateCanceled
	})
	assert.Equal(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation])
	assert.True(t, ops.PlanDispatchedBy(secret, ownerKey), "the canceled plan should still be the one the operation dispatched")
	assert.Equal(t, "snapshot", firstInstructionName(t, secret), "the restart plan must never have been dispatched")

	return secret
}

// setPlanPaused sets or lifts the pause on the fixture's etcd machine-plan, on behalf of the
// lifecycle hook delegate holding the cluster's beacon: the writer the machine-plan webhook checks
// such a write against.
func setPlanPaused(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, paused bool) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret := etcdPlanSecret(t, cs, fx).DeepCopy()
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		if paused {
			secret.Annotations[planapi.PlanPausedAnnotation] = "true"
		} else {
			delete(secret.Annotations, planapi.PlanPausedAnnotation)
		}
		secret.Annotations[planapi.PlanWriterAnnotation] = cancelDelegateName
		_, err := cs.Core.Secret().Update(secret)
		return err
	})
	require.NoError(t, err, "set %s=%t", planapi.PlanPausedAnnotation, paused)
}

// waitForPlanSecret polls the fixture's etcd machine-plan secret until done reports true for it.
func waitForPlanSecret(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, what string, done func(*corev1.Secret) bool) *corev1.Secret {
	t.Helper()

	var secret *corev1.Secret
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		secret = etcdPlanSecret(t, cs, fx)
		return done(secret), nil
	})
	if err != nil {
		handleError(t, cs, fx.mgmtCluster.Name, fmt.Errorf("waiting for %s (plan-state %q): %w", what, secret.Data[planapi.PlanStateKey], err))
	}
	return secret
}

// etcdPlanSecret returns the machine-plan secret of the fixture's single etcd node.
func etcdPlanSecret(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) *corev1.Secret {
	t.Helper()

	secrets, err := cs.Core.Secret().List(fx.mgmtCluster.Name, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=true", capr.ClusterNameLabel, fx.mgmtCluster.Name, capr.EtcdRoleLabel),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	require.NoError(t, err)
	require.Len(t, secrets.Items, 1, "expected exactly one etcd machine-plan secret")
	return &secrets.Items[0]
}

// firstInstructionName returns the name of the first one-time instruction of the plan on secret.
func firstInstructionName(t *testing.T, secret *corev1.Secret) string {
	t.Helper()

	assigned, err := planapi.Parse(secret.Data[planapi.PlanDataKey])
	require.NoError(t, err)
	require.NotEmpty(t, assigned.OneTimeInstructions)
	return assigned.OneTimeInstructions[0].Name
}

// planRevision returns the plan-revision the agent last recorded on secret.
func planRevision(t *testing.T, secret *corev1.Secret) int {
	t.Helper()

	raw := secret.Data[planapi.PlanRevisionKey]
	require.NotEmpty(t, raw, "plan-revision is missing: the agent does not support plan-state")
	revision, err := strconv.Atoi(string(raw))
	require.NoError(t, err)
	return revision
}
