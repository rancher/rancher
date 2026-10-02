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

// These tests depend on a system-agent which supports plan-state (and with it, plan cancellation).
// They are not part of any CI scope.

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveCancel cancels an ETCDSnapshotSave and
// asserts that the cancellation reaches the machine-plan secret rather than stopping at the
// operation's status: the plan the operation dispatched is annotated as canceled, and the agent
// records plan-state canceled for it.
//
// The operation is held on a Restart step hook so the cancellation lands at a deterministic point:
// the save plan has been applied, and the restart plan has not yet been dispatched. The agent records
// a cancellation against a succeeded plan as well as an in-flight one, since a succeeded plan still
// runs periodic instructions, so this does not have to race the snapshot itself.
func Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveCancel(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-snapshot-cancel", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	cancelSnapshotSaveAtRestart(t, cs, fx)
}

// Test_Imported_Operation_SetD_ImportedETCDSnapshotSaveFollowUp runs saves of the same shape one
// after another, each of which computes the same plan apart from the operation it is scoped to. Each
// one must be run by the agent on its own account rather than inheriting the outcome of the plan left
// on the machine-plan secret by the one before it:
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

	canceled := cancelSnapshotSaveAtRestart(t, cs, fx)
	revision := planRevision(t, canceled)

	for _, leg := range []string{"after a canceled save", "after a succeeded save"} {
		op := RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)

		secret := etcdPlanSecret(t, cs, fx)
		assert.True(t, ops.PlanDispatchedBy(secret, op), "%s: the plan on the secret should be the follow-up's own", leg)
		assert.Equal(t, string(planapi.PlanStateSucceeded), string(secret.Data[planapi.PlanStateKey]), leg)
		assert.NotEqual(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation],
			"%s: assigning new plan content must clear the previous plan's cancellation", leg)

		next := planRevision(t, secret)
		assert.GreaterOrEqual(t, next, revision+2,
			"%s: the agent should have picked up both the save and the restart plan of the follow-up", leg)
		revision = next
	}
}

// cancelSnapshotSaveAtRestart runs an ETCDSnapshotSave up to its Restart step hook, cancels it there,
// and waits for both the operation and the plan it dispatched to report the cancellation. Returns the
// machine-plan secret as it stands once the agent has recorded the plan as canceled.
func cancelSnapshotSaveAtRestart(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) *corev1.Secret {
	t.Helper()

	const (
		hookName     = "v2prov-e2e-cancel"
		delegateName = "v2prov-e2e-cancel-delegate"
	)
	restartHookKey := etcdsnapshotsave.RestartStepHookLabelPrefix + hookName
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	op := CreateETCDSnapshotSaveOp(t, cs, fx.ns.Name, fx.clusterRef, WithSaveLabels(map[string]string{
		restartHookKey: delegateName,
	}))

	WaitForSnapshotSaveHookPause(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepRestart)

	// The save step completed, so the plan on the secret is this operation's save plan, run to
	// completion, and the restart plan has not been dispatched.
	secret := etcdPlanSecret(t, cs, fx)
	require.True(t, ops.PlanDispatchedBy(secret, op), "the save plan should be on the secret while the operation waits on its restart hook")
	assert.Equal(t, "snapshot", firstInstructionName(t, secret), "the restart plan must not have been dispatched yet")
	assert.Equal(t, string(planapi.PlanStateSucceeded), string(secret.Data[planapi.PlanStateKey]))

	// Cancel before releasing the hook: every reconcile from here on sees the cancellation before it
	// would advance the operation, so the restart plan is never dispatched.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
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

	AdvancePastSnapshotSaveHook(t, cs, op, beaconNS, beaconName, restartHookKey, delegateName)

	var latest *opv1alpha1.ETCDSnapshotSave
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

	err = utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		secret = etcdPlanSecret(t, cs, fx)
		return planapi.PlanState(secret.Data[planapi.PlanStateKey]) == planapi.PlanStateCanceled, nil
	})
	if err != nil {
		handleError(t, cs, fx.mgmtCluster.Name, fmt.Errorf("waiting for the agent to record plan-state %s (have %q): %w",
			planapi.PlanStateCanceled, secret.Data[planapi.PlanStateKey], err))
	}

	assert.Equal(t, "true", secret.Annotations[planapi.PlanCanceledAnnotation])
	assert.True(t, ops.PlanDispatchedBy(secret, op), "the canceled plan should still be the one the operation dispatched")
	assert.Equal(t, "snapshot", firstInstructionName(t, secret), "the restart plan must never have been dispatched")

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
