package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/controllers/operations/encryptionkeyrotation"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotsave"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// createSaveExpectingRejection attempts to create an ETCDSnapshotSave and returns the admission error.
// It fails the test if the create is admitted, since that would mean an operation was allowed to queue
// up behind the beacon holder.
func createSaveExpectingRejection(t *testing.T, cs *clients.Clients, namespace string, clusterRef corev1.ObjectReference) error {
	t.Helper()

	op, err := cs.Operation.ETCDSnapshotSave().Create(&opv1alpha1.ETCDSnapshotSave{
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
	})
	if err == nil {
		_ = cs.Operation.ETCDSnapshotSave().Delete(op.Namespace, op.Name, &metav1.DeleteOptions{})
		t.Fatalf("expected the creation of %s/%s to be rejected while another operation held the beacon", op.Namespace, op.Name)
	}

	return err
}

// waitForEncryptionKeyRotationCanceled polls the EncryptionKeyRotation until it reaches the Canceled
// terminal phase. It fails fast if the operation instead reaches Succeeded or Failed. This is the
// observable signal that a restore operation preempted (canceled) the in-flight rotation.
func waitForEncryptionKeyRotationCanceled(t *testing.T, cs *clients.Clients, op *opv1alpha1.EncryptionKeyRotation) *opv1alpha1.EncryptionKeyRotation {
	t.Helper()

	var latestOp *opv1alpha1.EncryptionKeyRotation
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err := cs.Operation.EncryptionKeyRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch got.Status.Phase {
		case opv1alpha1.OperationPhaseSucceeded:
			return false, fmt.Errorf("operation %s/%s reached Succeeded but should have been canceled by the restore operation", got.Namespace, got.Name)
		case opv1alpha1.OperationPhaseFailed:
			return false, fmt.Errorf("operation %s/%s reached Failed at step %q but should have been canceled by the restore operation", got.Namespace, got.Name, got.Status.Step)
		case opv1alpha1.OperationPhaseCanceled:
			latestOp = got
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		handleEKRError(t, cs, op.Namespace, op.Spec.ClusterRef.Name, err)
	}
	return latestOp
}

// waitForRestoreRequiredCleared polls the management cluster until the restore-required annotation has
// been removed.
func waitForRestoreRequiredCleared(t *testing.T, cs *clients.Clients, clusterName string) {
	t.Helper()

	err := utilwait.PollUntilContextTimeout(cs.Ctx, 5*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		clusterObj, err := cs.Mgmt.Cluster().Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		_, marked := clusterObj.Annotations[opv1alpha1.RestoreRequiredAnnotation]
		return !marked, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for %s to be removed from cluster %s: %v", opv1alpha1.RestoreRequiredAnnotation, clusterName, err)
	}
}

// Test_Imported_Operation_SetE_ImportedETCDSnapshotSaveBeaconContention verifies that a second
// ETCDSnapshotSave cannot be created while another operation already holds the cluster beacon.
//
// It uses the etcdsnapshotsave lifecycle-hook mechanism to pause a first save at its Preflight step
// — at which point the save owns the beacon (owner set, active, delegate on the chain). With the
// beacon held, a second save is attempted; because a non-restore operation may not preempt an
// operation in flight, the webhook is expected to reject the create outright. The first save is then
// released and is expected to complete normally, proving the contention did not disturb the holder.
func Test_Imported_Operation_SetE_ImportedETCDSnapshotSaveBeaconContention(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-save-beacon-contention", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	hookName := "beacon-contention"
	delegateName := "beacon-contention-delegate"
	hookKey := etcdsnapshotsave.PreflightStepHookLabelPrefix + hookName

	// First save pauses at Preflight while holding the beacon.
	save1 := CreateETCDSnapshotSaveOp(t, cs, fx.ns.Name, fx.clusterRef, WithSaveLabels(map[string]string{
		hookKey: delegateName,
	}))
	WaitForSnapshotSaveHookPause(t, cs, save1, beaconNS, beaconName, hookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.ETCDSnapshotSaveStepPreflight)

	// A second save may not be created while the first is in flight.
	err = createSaveExpectingRejection(t, cs, fx.ns.Name, fx.clusterRef)
	assert.ErrorContains(t, err, save1.Name, "the rejection should name the operation which blocked the create")

	// A cancelled save does not require a restore, and neither does one which was never created.
	clusterObj, err := cs.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, clusterObj.Annotations, opv1alpha1.RestoreRequiredAnnotation)

	// Releasing the first save lets it complete normally after the contention.
	AdvancePastSnapshotSaveHook(t, cs, save1, beaconNS, beaconName, hookKey, delegateName)
	save1 = WaitForSnapshotSaveSucceeded(t, cs, save1, beaconNS, beaconName)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, save1.Status.Phase)
}

// Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreCancelsEncryptionKeyRotation verifies that
// an ETCDSnapshotRestore is allowed to preempt (cancel) an operation that already holds the beacon,
// whereas other operations may not even be created.
//
// The test first runs a snapshot save to produce a restorable snapshot, then starts an
// EncryptionKeyRotation paused at its Rotate step via a lifecycle hook — at which point the rotation
// holds the beacon (before any rotate-keys work, so this is distro-agnostic). A restore is then
// created for the snapshot. The rotation is expected to be Canceled (preempted) and to mark the
// cluster as requiring a restore, and the restore is expected to acquire the beacon, complete
// successfully, restore the deleted ConfigMap, and clear that mark.
func Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreCancelsEncryptionKeyRotation(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-restore-preempts-ekr", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})

	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	// Payload whose presence after the restore proves the snapshot was actually rolled back.
	cm := newConfigMapProof()
	cm.create(t, fx)

	// Produce a snapshot to restore from.
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, 1)
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)

	// Delete the ConfigMap so the post-restore assertion is meaningful.
	cm.delete(t, fx)

	// Start an encryption key rotation and pause it at Rotate while it holds the beacon.
	hookName := "beacon-preempt"
	delegateName := "beacon-preempt-delegate"
	ekrHookKey := encryptionkeyrotation.RotateStepHookLabelPrefix + hookName

	ekr := CreateEncryptionKeyRotationOp(t, cs, fx.ns.Name, fx.clusterRef, WithEncryptionKeyRotationLabels(map[string]string{
		ekrHookKey: delegateName,
	}))
	WaitForEncryptionKeyRotationHookPause(t, cs, ekr, beaconNS, beaconName, ekrHookKey, delegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.EncryptionKeyRotationStepRotate)

	// Create the restore (does not wait). It is expected to preempt the rotation and take the beacon.
	restore := CreateETCDSnapshotRestoreOp(t, cs, fx.ns.Name, snapshot.SnapshotFile.Name, fx.clusterRef)

	// The rotation must be canceled by the restore, unlike a save which could not have been created.
	ekr = waitForEncryptionKeyRotationCanceled(t, cs, ekr)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, ekr.Status.Phase)
	assert.Equal(t, opv1alpha1.PreemptedReason, opv1alpha1.CanceledCondition.GetReason(ekr))

	// Cancelling a rotation part-way through leaves the cluster needing a restore, and that is
	// recorded on the cluster the operation referenced.
	clusterObj, err := cs.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.ObjectRefKey("EncryptionKeyRotation", ekr.Namespace, ekr.Name),
		clusterObj.Annotations[opv1alpha1.RestoreRequiredAnnotation])

	// The restore takes the beacon and runs to completion. WaitForSnapshotRestoreSucceeded only
	// allows 10 minutes, which is too short for a full restore, so wait with the same 20-minute
	// window RunETCDSnapshotRestoreOperationTest uses.
	err = wait.ObjectWithTimeout(cs.Ctx, 20*time.Minute, cs.Operation.ETCDSnapshotRestore().Watch, restore, func(obj runtime.Object) (bool, error) {
		restore = obj.(*opv1alpha1.ETCDSnapshotRestore)
		if restore.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("etcd snapshot restore operation failed at step %q", restore.Status.Step)
		}
		return restore.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		handleError(t, cs, fx.clusterRef.Name, err)
	}
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, restore.Status.Phase)

	// The restored etcd state must contain the ConfigMap we deleted before the restore.
	cm.assertRestored(t, fx)

	// A successful restore returns the cluster to a known-good state, so the mark is removed.
	waitForRestoreRequiredCleared(t, cs, fx.mgmtCluster.Name)

	// Best-effort cleanup: the canceled rotation still carries its rotate-step hook label, which
	// defers TTL garbage collection. Its beacon delegate was already cleared when the rotation
	// released the beacon on cancellation, so only the label needs removing.
	if latest, getErr := cs.Operation.EncryptionKeyRotation().Get(ekr.Namespace, ekr.Name, metav1.GetOptions{}); getErr == nil {
		latest = latest.DeepCopy()
		delete(latest.Labels, ekrHookKey)
		_, _ = cs.Operation.EncryptionKeyRotation().Update(latest)
	}
}
