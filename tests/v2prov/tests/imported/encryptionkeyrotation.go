package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// EncryptionKeyRotationOption mutates the EncryptionKeyRotation object before it is submitted.
// Mirrors the SnapshotSaveOption / SnapshotRestoreOption pattern — use it to attach lifecycle-hook
// labels for tests that want to walk the operation through each step.
type EncryptionKeyRotationOption func(*opv1alpha1.EncryptionKeyRotation)

// WithEncryptionKeyRotationLabels merges the given labels onto the operation. Primarily used to
// drive the lifecycle-hook codepath in the encryptionkeyrotation controller.
func WithEncryptionKeyRotationLabels(labels map[string]string) EncryptionKeyRotationOption {
	return func(op *opv1alpha1.EncryptionKeyRotation) {
		if op.Labels == nil {
			op.Labels = map[string]string{}
		}
		for k, v := range labels {
			op.Labels[k] = v
		}
	}
}

// buildEncryptionKeyRotationOp assembles the operation object with the standard defaults so the
// Create/Run/Hook helpers don't duplicate the literal.
func buildEncryptionKeyRotationOp(namespace string, clusterRef corev1.ObjectReference, opts ...EncryptionKeyRotationOption) *opv1alpha1.EncryptionKeyRotation {
	op := &opv1alpha1.EncryptionKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-ekr-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.EncryptionKeyRotationSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
			},
		},
	}
	for _, opt := range opts {
		opt(op)
	}
	return op
}

// RunEncryptionKeyRotationOperationTest creates an EncryptionKeyRotation operation targeting the
// given clusterRef and waits for it to reach the Succeeded phase. This tests the
// operation.cattle.io/v1alpha1 EncryptionKeyRotation controller which manages key rotation
// lifecycle through beacons and plan secrets. Pass options (e.g. WithEncryptionKeyRotationLabels)
// to mutate the operation before creation — useful for driving the lifecycle-hook codepath.
func RunEncryptionKeyRotationOperationTest(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...EncryptionKeyRotationOption) *opv1alpha1.EncryptionKeyRotation {
	t.Helper()

	op, err := clients.Operation.EncryptionKeyRotation().Create(buildEncryptionKeyRotationOp(namespace, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}

	err = wait.ObjectWithTimeout(clients.Ctx, 25*time.Minute, clients.Operation.EncryptionKeyRotation().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.EncryptionKeyRotation)
		if op.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("encryption key rotation operation failed at step %q", op.Status.Step)
		}
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		handleEKRError(t, clients, namespace, clusterRef.Name, err)
	}

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)

	err = wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Operation.EncryptionKeyRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
	})
	if err != nil {
		t.Fatalf("timed out waiting for operation %s %s/%s to be deleted: %v", opv1alpha1.EncryptionKeyRotationResourceName, op.Namespace, op.Name, err)
	}

	return op
}

func handleEKRError(t *testing.T, clients *clients.Clients, namespace, name string, err error) {
	if err != nil {
		objs := map[string]any{}

		c, newErr := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["mgmtCluster"] = c

			nodes, newErr := clients.Mgmt.Node().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["mgmtNodes"] = nodes
			}

			beacon, newErr := clients.Plan.Beacon().Get(c.Name, c.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["beacon"] = beacon
			}

			secrets, newErr := clients.Core.Secret().List(c.Name, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name),
				FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
			})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["machinePlans"] = secrets
			}

			ekrs, newErr := clients.Operation.EncryptionKeyRotation().List(namespace, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["EncryptionKeyRotation"] = ekrs
			}
		}

		features, newErr := clients.Mgmt.Feature().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["features"] = features
		}

		settings, newErr := clients.Mgmt.Setting().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["settings"] = settings
		}

		data, newErr := snapshotutil.CompressInterface(objs)
		if newErr != nil {
			logrus.Error(newErr)
		}
		//nolint:revive
		err = fmt.Errorf("cluster %s encryption key rotation wait failed on: %w\ncluster %s test data bundle: \n%s\n", name, err, name, data)
		t.Fatal(err)
	}
}

// CreateEncryptionKeyRotationOp creates an EncryptionKeyRotation but does NOT wait for it to
// complete. Mirrors CreateETCDSnapshotSaveOp — use this in hook-driven tests where the operation
// is inspected/advanced one checkpoint at a time via WaitForEncryptionKeyRotationHookPause +
// AdvancePastEncryptionKeyRotationHook.
func CreateEncryptionKeyRotationOp(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference, opts ...EncryptionKeyRotationOption) *opv1alpha1.EncryptionKeyRotation {
	t.Helper()
	op, err := clients.Operation.EncryptionKeyRotation().Create(buildEncryptionKeyRotationOp(namespace, clusterRef, opts...))
	if err != nil {
		t.Fatal(err)
	}
	return op
}

// EncryptionKeyRotationCheckpoint is the state captured when WaitForEncryptionKeyRotationHookPause
// confirms a hook has fired. Op is the latest operation object (labels, status, conditions);
// Beacon is the latest beacon (delegate chain, owner, active flag).
type EncryptionKeyRotationCheckpoint struct {
	Op     *opv1alpha1.EncryptionKeyRotation
	Beacon *planv1alpha1.Beacon
}

// WaitForEncryptionKeyRotationHookPause polls until the named hook on the op has clearly fired:
// the op is at the expected (phase, step), the hook label is still present, and the named
// delegate is on the beacon's chain. Returns the captured state so the caller can inspect
// anything between pause and release — e.g. the assigned rotate-keys / restart plan, periodic
// status output, or the pause flag on the CAPI cluster.
//
// expectedStep may be empty for phase-only hooks (Pending/Canceled/Failed/Succeeded). Bails fast
// if the op reaches Failed before the checkpoint is hit.
func WaitForEncryptionKeyRotationHookPause(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.EncryptionKeyRotation,
	beaconNS, beaconName, hookLabelKey, delegateName string,
	expectedPhase opv1alpha1.OperationPhase,
	expectedStep opv1alpha1.EncryptionKeyRotationStep,
) EncryptionKeyRotationCheckpoint {
	t.Helper()

	// 25-minute window matches RunEncryptionKeyRotationOperationTest's timeout — rotate-keys +
	// per-node restart is the longest of the three operation types, so even one checkpoint can
	// sit pending for a while in a multi-node topology.
	var checkpoint EncryptionKeyRotationCheckpoint
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 25*time.Minute, true, func(_ context.Context) (bool, error) {
		latestOp, err := clients.Operation.EncryptionKeyRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
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
		checkpoint = EncryptionKeyRotationCheckpoint{Op: latestOp, Beacon: beacon}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for hook %q to pause op %s/%s at phase=%q step=%q: %v",
			hookLabelKey, op.Namespace, op.Name, expectedPhase, expectedStep, err)
	}
	return checkpoint
}

// AdvancePastEncryptionKeyRotationHook clears the hook label and pops the delegate, in that order.
// Both are required: clearing the label alone leaves the delegate on the chain; popping alone
// leaves the label, so the next reconcile re-pushes the same delegate and deadlocks the op.
func AdvancePastEncryptionKeyRotationHook(
	t *testing.T,
	clients *clients.Clients,
	op *opv1alpha1.EncryptionKeyRotation,
	beaconNS, beaconName, hookLabelKey, delegateName string,
) {
	t.Helper()

	latestOp, err := clients.Operation.EncryptionKeyRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get op %s/%s: %v", op.Namespace, op.Name, err)
	}
	latestOp = latestOp.DeepCopy()
	delete(latestOp.Labels, hookLabelKey)
	if _, err := clients.Operation.EncryptionKeyRotation().Update(latestOp); err != nil {
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

// WaitForEncryptionKeyRotationSucceeded polls until the op reaches Succeeded with no delegate
// left on the beacon. Use this as the final assertion in hook tests after all hook labels have
// been cleared.
func WaitForEncryptionKeyRotationSucceeded(t *testing.T, clients *clients.Clients, op *opv1alpha1.EncryptionKeyRotation, beaconNS, beaconName string) *opv1alpha1.EncryptionKeyRotation {
	t.Helper()

	var latestOp *opv1alpha1.EncryptionKeyRotation
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 10*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := clients.Operation.EncryptionKeyRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
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
