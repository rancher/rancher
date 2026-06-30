package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

func waitForSnapshots(t *testing.T, clients *clients.Clients, clusterName string, createdAfter time.Time, desired int) {
	t.Helper()

	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		list, err := clients.RKE.ETCDSnapshot().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, clusterName),
		})
		if err != nil {
			return false, err
		}
		// Prefer the newest snapshot that landed after we started saving; this avoids picking up a
		// stale snapshot from a prior test run that somehow shares this cluster's namespace.
		var count int
		for i := range list.Items {
			s := &list.Items[i]
			if s.SnapshotFile.CreatedAt == nil || !s.SnapshotFile.CreatedAt.Time.After(createdAfter) {
				continue
			}
			if s.SnapshotFile.Name == "" {
				continue
			}
			count++
		}
		return desired == count, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for back-populated ETCDSnapshot CR: %v", err)
	}
}

// waitForBackpopulatedSnapshot polls until at least one ETCDSnapshot CR has been back-populated for
// the imported cluster, then returns the most recently created one. The snapshotbackpopulate
// controller writes these CRs into the namespace whose name matches the cluster (cluster-scoped
// mgmt clusters use namespace == name).
func waitForBackpopulatedSnapshot(t *testing.T, clients *clients.Clients, clusterName, nodeName string, createdAfter time.Time) *rkev1.ETCDSnapshot {
	t.Helper()

	var picked *rkev1.ETCDSnapshot
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		list, err := clients.RKE.ETCDSnapshot().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s", capr.ClusterNameLabel, clusterName, capr.NodeNameLabel, nodeName),
		})
		if err != nil {
			return false, err
		}
		// Prefer the newest snapshot that landed after we started saving; this avoids picking up a
		// stale snapshot from a prior test run that somehow shares this cluster's namespace.
		for i := range list.Items {
			s := &list.Items[i]
			if s.SnapshotFile.CreatedAt == nil || !s.SnapshotFile.CreatedAt.Time.After(createdAfter) {
				continue
			}
			if s.SnapshotFile.Name == "" {
				continue
			}
			if picked == nil || s.SnapshotFile.CreatedAt.After(picked.SnapshotFile.CreatedAt.Time) {
				picked = s
			}
		}
		return picked != nil, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for back-populated ETCDSnapshot CR: %v", err)
	}
	return picked
}

// SnapshotRestoreOption mutates the ETCDSnapshotRestore object before it is submitted. Mirrors
// SnapshotSaveOption — use it to attach lifecycle-hook labels or override the default TTL.
type SnapshotRestoreOption func(*opv1alpha1.ETCDSnapshotRestore)

// WithRestoreLabels merges the given labels onto the ETCDSnapshotRestore. Primarily used to drive
// the lifecycle-hook codepath in the etcdsnapshotrestore controller, which delegates the beacon
// when any `<phase>.phase.hook.operation.cattle.io/<name>` (or step-equivalent) label is set.
func WithRestoreLabels(labels map[string]string) SnapshotRestoreOption {
	return func(op *opv1alpha1.ETCDSnapshotRestore) {
		if op.Labels == nil {
			op.Labels = map[string]string{}
		}
		for k, v := range labels {
			op.Labels[k] = v
		}
	}
}

// buildSnapshotRestoreOp assembles the ETCDSnapshotRestore object with the standard defaults so the
// Create/Run/Hook helpers don't duplicate the literal.
func buildSnapshotRestoreOp(namespace, snapshotName string, clusterRef corev1.ObjectReference, opts ...SnapshotRestoreOption) *opv1alpha1.ETCDSnapshotRestore {
	op := &opv1alpha1.ETCDSnapshotRestore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-restore-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.ETCDSnapshotRestoreSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
				TTL:        60,
			},
			Args: opv1alpha1.ETCDSnapshotRestoreArgs{
				Name: snapshotName,
			},
		},
	}
	for _, opt := range opts {
		opt(op)
	}
	return op
}

// RunETCDSnapshotRestoreOperationTest creates an ETCDSnapshotRestore operation targeting the given
// clusterRef and waits for it to reach the Succeeded phase. Mirrors RunETCDSnapshotSaveOperationTest
// — exercises the operation.cattle.io/v1alpha1 ETCDSnapshotRestore controller end-to-end. Pass
// options (e.g. WithRestoreLabels) to mutate the operation object before creation.
func RunETCDSnapshotRestoreOperationTest(t *testing.T, clients *clients.Clients, namespace, snapshotName string, clusterRef corev1.ObjectReference, opts ...SnapshotRestoreOption) *opv1alpha1.ETCDSnapshotRestore {
	t.Helper()

	op, err := clients.Operation.ETCDSnapshotRestore().Create(buildSnapshotRestoreOp(namespace, snapshotName, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}

	// Restore is multi-phase (shutdown → restore → pod cleanup → restart → node cleanup → restart),
	// each phase waits on the agent to apply its plan, and the etcd reset itself is several minutes.
	// 20 minutes is generous but matches what the in-place provisioning restore test allows for.
	err = wait.ObjectWithTimeout(clients.Ctx, 20*time.Minute, clients.Operation.ETCDSnapshotRestore().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.ETCDSnapshotRestore)
		if op.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("etcd snapshot restore operation failed at step %q", op.Status.Step)
		}
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		handleError(t, clients, clusterRef.Name, err)
	}

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	return op
}

// CreateETCDSnapshotRestoreOp creates an ETCDSnapshotRestore but does NOT wait for it to complete.
// Mirrors CreateETCDSnapshotSaveOp — use this in hook-driven tests where the operation should be
// inspected/advanced one checkpoint at a time via WaitForSnapshotRestoreHookPause +
// AdvancePastSnapshotRestoreHook.
func CreateETCDSnapshotRestoreOp(t *testing.T, clients *clients.Clients, namespace, snapshotName string, clusterRef corev1.ObjectReference, opts ...SnapshotRestoreOption) *opv1alpha1.ETCDSnapshotRestore {
	t.Helper()
	op, err := clients.Operation.ETCDSnapshotRestore().Create(buildSnapshotRestoreOp(namespace, snapshotName, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// SnapshotRestoreCheckpoint is the state captured when WaitForSnapshotRestoreHookPause confirms a
// hook has fired. Mirrors SnapshotSaveCheckpoint — Op is the current operation object (latest
// labels, status, conditions), Beacon is the current beacon (delegate chain, owner, active flag).
type SnapshotRestoreCheckpoint struct {
	Op     *opv1alpha1.ETCDSnapshotRestore
	Beacon *planv1alpha1.Beacon
}

// WaitForSnapshotRestoreHookPause polls until the named hook on the op has clearly fired: the op
// is at the expected (phase, step), the hook label is still present, and the named delegate is on
// the beacon's chain. Returns the captured state so the caller can inspect anything between pause
// and release (plan secrets, op conditions, beacon details) — see the test godoc for richer use
// cases (e.g. validating shutdown plan contents, simulating partial failures).
//
// expectedStep may be empty for phase-only hooks (Pending/Canceled/Failed/Succeeded). Bails fast
// if the op reaches Failed before the checkpoint is hit.
func WaitForSnapshotRestoreHookPause(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.ETCDSnapshotRestore,
	beaconNS, beaconName, hookLabelKey, delegateName string,
	expectedPhase opv1alpha1.OperationPhase,
	expectedStep opv1alpha1.ETCDSnapshotRestoreStep,
) SnapshotRestoreCheckpoint {
	t.Helper()

	// 20-minute window matches RunETCDSnapshotRestoreOperationTest's timeout — restore steps are
	// long-running (etcd reset, restart waits) so the slowest checkpoint can sit pending for a while.
	var checkpoint SnapshotRestoreCheckpoint
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 20*time.Minute, true, func(_ context.Context) (bool, error) {
		latestOp, err := clients.Operation.ETCDSnapshotRestore().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
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
		checkpoint = SnapshotRestoreCheckpoint{Op: latestOp, Beacon: beacon}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for hook %q to pause op %s/%s at phase=%q step=%q: %v",
			hookLabelKey, op.Namespace, op.Name, expectedPhase, expectedStep, err)
	}
	return checkpoint
}

// AdvancePastSnapshotRestoreHook clears the hook label and pops the delegate, in that order. Both
// are required: clearing the label alone leaves the delegate on the chain (the next reconcile
// would still see it via beacon authority logic); popping alone leaves the label, so the next
// reconcile re-pushes the same delegate and the operation deadlocks.
func AdvancePastSnapshotRestoreHook(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.ETCDSnapshotRestore,
	beaconNS, beaconName, hookLabelKey, delegateName string,
) {
	t.Helper()

	latestOp, err := clients.Operation.ETCDSnapshotRestore().Get(op.Namespace, op.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get op %s/%s: %v", op.Namespace, op.Name, err)
	}
	latestOp = latestOp.DeepCopy()
	delete(latestOp.Labels, hookLabelKey)
	if _, err := clients.Operation.ETCDSnapshotRestore().Update(latestOp); err != nil {
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

// WaitForSnapshotRestoreSucceeded polls until the op reaches Succeeded with no delegate left on
// the beacon. Use this as the final assertion in hook tests after all hook labels have been
// cleared — tolerates the brief delegate-held window between popping the last hook and the
// controller draining the chain.
func WaitForSnapshotRestoreSucceeded(t *testing.T, clients *clients.Clients, op *opv1alpha1.ETCDSnapshotRestore, beaconNS, beaconName string) *opv1alpha1.ETCDSnapshotRestore {
	t.Helper()

	var latestOp *opv1alpha1.ETCDSnapshotRestore
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 10*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := clients.Operation.ETCDSnapshotRestore().Get(op.Namespace, op.Name, metav1.GetOptions{})
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
