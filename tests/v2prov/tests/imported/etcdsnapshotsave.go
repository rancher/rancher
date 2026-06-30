package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// SnapshotSaveOption mutates the ETCDSnapshotSave object before it is submitted. Use this to
// attach labels (e.g. lifecycle-hook prefixes), tweak the TTL, or override the GenerateName for
// scenarios that need to inspect the object after creation.
type SnapshotSaveOption func(*opv1alpha1.ETCDSnapshotSave)

// WithSaveLabels merges the given labels onto the ETCDSnapshotSave. Primarily used to drive the
// lifecycle-hook codepath in the etcdsnapshotsave controller, which delegates the beacon when any
// `<phase>.phase.hook.operation.cattle.io/<name>` (or step-equivalent) label is set.
func WithSaveLabels(labels map[string]string) SnapshotSaveOption {
	return func(op *opv1alpha1.ETCDSnapshotSave) {
		if op.Labels == nil {
			op.Labels = map[string]string{}
		}
		for k, v := range labels {
			op.Labels[k] = v
		}
	}
}

// RunETCDSnapshotSaveOperationTest creates an ETCDSnapshotSave operation targeting the given
// clusterRef and waits for it to reach the Succeeded phase. This tests the operation.cattle.io/v1alpha1
// ETCDSnapshotSave controller which manages snapshot lifecycle through beacons and plan secrets.
// Pass options (e.g. WithSaveLabels) to mutate the operation object before creation — useful for
// driving non-default code paths like lifecycle hooks.
func RunETCDSnapshotSaveOperationTest(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...SnapshotSaveOption) *opv1alpha1.ETCDSnapshotSave {
	t.Helper()

	op := &opv1alpha1.ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-snapshot-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.ETCDSnapshotSaveSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
				TTL:        60,
			},
		},
	}

	for _, opt := range opts {
		opt(op)
	}

	op, err := clients.Operation.ETCDSnapshotSave().Create(op)
	if err != nil {
		t.Fatal(err)
	}

	err = wait.ObjectWithTimeout(clients.Ctx, 5*time.Minute, clients.Operation.ETCDSnapshotSave().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.ETCDSnapshotSave)
		if op.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("etcd snapshot create operation failed at step %q", op.Status.Step)
		}
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		handleError(t, clients, clusterRef.Name, err)
	}

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	return op
}

// CreateETCDSnapshotSaveOp creates an ETCDSnapshotSave but does NOT wait for it to complete. This
// is the entry point for lifecycle-hook-driven tests that need to inspect or manipulate state at
// intermediate steps — callers thread the returned op through WaitForSnapshotSaveHookPause and
// AdvancePastSnapshotSaveHook to walk the operation forward one checkpoint at a time. Use
// RunETCDSnapshotSaveOperationTest instead when you just want the snapshot to succeed.
func CreateETCDSnapshotSaveOp(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...SnapshotSaveOption) *opv1alpha1.ETCDSnapshotSave {
	t.Helper()

	op := &opv1alpha1.ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-snapshot-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.ETCDSnapshotSaveSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
				TTL:        60,
			},
		},
	}

	for _, opt := range opts {
		opt(op)
	}

	op, err := clients.Operation.ETCDSnapshotSave().Create(op)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// SnapshotSaveCheckpoint is the state captured by WaitForSnapshotSaveHookPause at the moment the
// controller pauses on a hook. The fields are intentionally narrow: the op (so callers can read
// status conditions / labels / step), and the beacon (so callers can confirm the delegate is in
// the chain and inspect any other beacon state). Plan secrets are fetched separately by tests
// that need them, because the relevant label selector is operation-specific.
type SnapshotSaveCheckpoint struct {
	Op     *opv1alpha1.ETCDSnapshotSave
	Beacon *planv1alpha1.Beacon
}

// WaitForSnapshotSaveHookPause polls until the named hook on the op has clearly fired:
//
//  1. the op's status reflects the expected phase (and step, when non-empty);
//  2. the hook label key is still present on the op (a delegate that already removed itself would
//     race the check — if the label is gone, we treat the checkpoint as missed and keep polling
//     in case the test caught a stale read);
//  3. the named delegate appears on the beacon's delegate chain.
//
// Returns the snapshot of state once all three are true so the caller can inspect anything it
// likes (plan secrets, status conditions, beacon contents) before calling
// AdvancePastSnapshotSaveHook to release the op. expectedStep may be empty for phase-only hooks
// (Pending/Canceled/Failed/Succeeded), in which case the step is not asserted.
func WaitForSnapshotSaveHookPause(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.ETCDSnapshotSave,
	beaconNS, beaconName, hookLabelKey, delegateName string,
	expectedPhase opv1alpha1.OperationPhase,
	expectedStep opv1alpha1.ETCDSnapshotSaveStep,
) SnapshotSaveCheckpoint {
	t.Helper()

	var checkpoint SnapshotSaveCheckpoint
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 2*time.Second, 10*time.Minute, true, func(_ context.Context) (bool, error) {
		latestOp, err := clients.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// A terminal Failed phase before reaching the checkpoint means the controller bailed —
		// surface that immediately rather than waiting out the full timeout.
		if latestOp.Status.Phase == opv1alpha1.OperationPhaseFailed && expectedPhase != opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("operation %s/%s reached Failed phase before hook %q fired: step=%q",
				latestOp.Namespace, latestOp.Name, hookLabelKey, latestOp.Status.Step)
		}
		if latestOp.Status.Phase != expectedPhase {
			return false, nil
		}
		if expectedStep != "" && latestOp.Status.Step != expectedStep {
			return false, nil
		}
		if _, ok := latestOp.Labels[hookLabelKey]; !ok {
			return false, nil
		}
		beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !plan.IsInDelegateChain(beacon, delegateName) {
			return false, nil
		}
		checkpoint = SnapshotSaveCheckpoint{Op: latestOp, Beacon: beacon}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for hook %q to pause op %s/%s at phase=%q step=%q: %v",
			hookLabelKey, op.Namespace, op.Name, expectedPhase, expectedStep, err)
	}
	return checkpoint
}

// AdvancePastSnapshotSaveHook clears the hook label and pops the delegate, in that order. Both
// steps are required to release the operation: clearing the label tells the controller's next
// reconcile that the hook is satisfied, popping the delegate cedes beacon authority back to the
// owning controller. Reversing the order would let the next reconcile re-push the same delegate
// (because the label is still present) before we can pop it, deadlocking the op.
func AdvancePastSnapshotSaveHook(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.ETCDSnapshotSave,
	beaconNS, beaconName, hookLabelKey, delegateName string,
) {
	t.Helper()

	latestOp, err := clients.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get op %s/%s: %v", op.Namespace, op.Name, err)
	}
	latestOp = latestOp.DeepCopy()
	delete(latestOp.Labels, hookLabelKey)
	if _, err := clients.Operation.ETCDSnapshotSave().Update(latestOp); err != nil {
		t.Fatalf("clear hook label %q on op %s/%s: %v", hookLabelKey, op.Namespace, op.Name, err)
	}

	beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get beacon %s/%s: %v", beaconNS, beaconName, err)
	}
	if _, err := plan.PopDelegate(beacon, delegateName, clients.Plan.Beacon()); err != nil {
		t.Fatalf("pop delegate %q from beacon %s/%s: %v", delegateName, beaconNS, beaconName, err)
	}
}

// WaitForSnapshotSaveSucceeded polls the op until it reaches Succeeded with no delegate left on
// the beacon — the all-hooks-released terminal state. Mirrors the wait performed by
// RunETCDSnapshotSaveOperationTest but tolerates the initial delegate-held window that hook tests
// produce.
func WaitForSnapshotSaveSucceeded(t *testing.T, clients *clients.Clients, op *opv1alpha1.ETCDSnapshotSave, beaconNS, beaconName string) *opv1alpha1.ETCDSnapshotSave {
	t.Helper()

	var latestOp *opv1alpha1.ETCDSnapshotSave
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := clients.Operation.ETCDSnapshotSave().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if got.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("operation reached Failed phase at step %q", got.Status.Step)
		}
		if got.Status.Phase != opv1alpha1.OperationPhaseSucceeded {
			return false, nil
		}
		beacon, err := clients.Plan.Beacon().Get(beaconNS, beaconName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// Once Succeeded and the delegate chain is drained, the owning controller's next reconcile
		// will release the beacon. We treat "delegate chain empty" as the success signal.
		if len(beacon.Status.Delegates) > 0 {
			return false, nil
		}
		latestOp = got
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for op %s/%s to reach Succeeded with empty delegate chain: %v", op.Namespace, op.Name, err)
	}
	return latestOp
}
