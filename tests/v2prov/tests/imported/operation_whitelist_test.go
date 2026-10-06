package imported

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/rancher/rancher/pkg/controllers/operations/certificaterotation"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// These tests depend on a system-agent which supports plan-state (and with it, plan pause and
// cancellation). They are not part of any CI scope.

const (
	whitelistHookName     = "v2prov-e2e-whitelist"
	whitelistDelegateName = "v2prov-e2e-whitelist-delegate"
)

// Test_Imported_Operation_SetD_ImportedCanceledRotationRequiresRestore cancels a certificate rotation
// after its point of no return, and follows the cluster it leaves behind until a restore repairs it:
//
//   - the rotation paused the cluster on leaving Preflight and whitelisted restores in the same write,
//     and canceling it leaves both, saying so on Finalized with RestoreRequired;
//   - a save is then turned away, by the operation webhook if it checks the whitelist, or otherwise by
//     its own preflight, and its rejection leaves the cluster paused;
//   - a restore is admitted and succeeds, unpausing the cluster and removing the whitelist;
//   - after which any operation may run again.
//
// The rotation is held at its Rotate step hook, after the pause and before it assigns any plan, so
// the cancellation is deterministic and the cluster has nothing to recover from but the pause.
func Test_Imported_Operation_SetD_ImportedCanceledRotationRequiresRestore(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-whitelist-restore", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	// The snapshot the restore repairs the cluster from, taken while any operation may still run.
	cm := newConfigMapProof()
	cm.create(t, fx)
	snapshotsValidAfter := time.Now().Add(-30 * time.Second)
	RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
	waitForSnapshots(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, snapshotsValidAfter, 1)
	snapshot := waitForBackpopulatedSnapshot(t, cs, fx.mgmtCluster.Name, fx.mgmtCluster.Name, "imported-init-0", snapshotsValidAfter)
	cm.delete(t, fx)

	rotateHookKey := certificaterotation.RotateStepHookLabelPrefix + whitelistHookName
	rotation := CreateCertificateRotationOp(t, cs, fx.ns.Name, fx.clusterRef, WithCertificateRotationLabels(map[string]string{
		rotateHookKey: whitelistDelegateName,
	}))
	WaitForCertificateRotationHookPause(t, cs, rotation, beaconNS, beaconName, rotateHookKey, whitelistDelegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.CertificateRotationStepRotate)
	assertClusterSettled(t, cs, fx, true, true, "a rotation past Preflight has paused and whitelisted the cluster")

	cancelCertificateRotation(t, cs, rotation)
	rotation = waitForCertificateRotationTerminated(t, cs, rotation)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, rotation.Status.Phase)
	assert.Equal(t, opv1alpha1.RestoreRequiredReason, opv1alpha1.FinalizedCondition.GetReason(&rotation.Status),
		"a rotation canceled past its point of no return says it left the cluster requiring a restore")
	assertClusterSettled(t, cs, fx, true, true, "a rotation canceled past its point of no return leaves the cluster paused and whitelisted")

	assertSaveTurnedAway(t, cs, fx)
	assertClusterSettled(t, cs, fx, true, true, "a rejected save leaves a whitelisted cluster paused")

	restore := RunETCDSnapshotRestoreOperationTest(t, cs, fx.ns.Name, snapshot.Name, fx.clusterRef)
	t.Logf("snapshot restore operation %s/%s completed", restore.Namespace, restore.Name)
	cm.assertRestored(t, fx)
	assertClusterSettled(t, cs, fx, false, false, "a succeeded restore unpauses the cluster and removes the whitelist")

	RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
}

// Test_Imported_Operation_SetD_ImportedRotationCanceledInPreflightLeavesClusterAlone cancels a
// certificate rotation while it is still in Preflight, before its point of no return. It has changed
// nothing on the cluster, so it leaves the cluster neither paused nor whitelisted, and the next
// operation runs as if it had never been created.
func Test_Imported_Operation_SetD_ImportedRotationCanceledInPreflightLeavesClusterAlone(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	fx := setUpImportedCluster(t, cs, "test-imported-whitelist-preflight", []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})
	beaconNS, beaconName := fx.mgmtCluster.Name, fx.mgmtCluster.Name

	preflightHookKey := certificaterotation.PreflightStepHookLabelPrefix + whitelistHookName
	rotation := CreateCertificateRotationOp(t, cs, fx.ns.Name, fx.clusterRef, WithCertificateRotationLabels(map[string]string{
		preflightHookKey: whitelistDelegateName,
	}))
	WaitForCertificateRotationHookPause(t, cs, rotation, beaconNS, beaconName, preflightHookKey, whitelistDelegateName,
		opv1alpha1.OperationPhaseInProgress, opv1alpha1.CertificateRotationStepPreflight)
	assertClusterSettled(t, cs, fx, false, false, "a rotation in Preflight has not changed the cluster yet")

	cancelCertificateRotation(t, cs, rotation)
	rotation = waitForCertificateRotationTerminated(t, cs, rotation)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, rotation.Status.Phase)
	assert.Equal(t, opv1alpha1.CertificateRotationStepPreflight, rotation.Status.Step)
	assert.NotEqual(t, opv1alpha1.RestoreRequiredReason, opv1alpha1.FinalizedCondition.GetReason(&rotation.Status))
	assertClusterSettled(t, cs, fx, false, false, "a rotation canceled in Preflight leaves the cluster as it was")

	RunETCDSnapshotSaveOperationTest(t, cs, fx.ns.Name, fx.clusterRef)
}

// assertClusterSettled asserts whether the imported cluster's mgmt Cluster is paused (version
// management paused) and whether it carries the operation whitelist, which is only ever the restores
// entry the operations add.
func assertClusterSettled(t *testing.T, cs *clients.Clients, fx *importedClusterFixture, paused, whitelisted bool, msg string) {
	t.Helper()

	c, err := cs.Mgmt.Cluster().Get(fx.mgmtCluster.Name, metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, paused, importedclusterversionmanagement.Paused(c), "%s: paused", msg)
	if whitelisted {
		assert.Equal(t, opv1alpha1.ETCDSnapshotRestoreResource, c.Annotations[opv1alpha1.WhitelistedAnnotation], "%s: whitelist", msg)
	} else {
		assert.NotContains(t, c.Annotations, opv1alpha1.WhitelistedAnnotation, "%s: whitelist", msg)
	}
}

// assertSaveTurnedAway asserts that a save can't run on the whitelisted cluster. The operation webhook
// refuses to create it when it checks the whitelist; without that check, the save is created and its
// preflight rejects it, holding the beacon, before it assigns anything. Either is a save turned away.
func assertSaveTurnedAway(t *testing.T, cs *clients.Clients, fx *importedClusterFixture) {
	t.Helper()

	save := buildSnapshotSaveOp(fx.ns.Name, fx.clusterRef)
	save, err := cs.Operation.ETCDSnapshotSave().Create(save)
	if err != nil {
		require.Contains(t, err.Error(), "only permits "+opv1alpha1.ETCDSnapshotRestoreResource,
			"a save may only be refused for the cluster's whitelist")
		return
	}

	var got *opv1alpha1.ETCDSnapshotSave
	err = utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		got, err = cs.Operation.ETCDSnapshotSave().Get(save.Namespace, save.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch got.Status.Phase {
		case opv1alpha1.OperationPhaseSucceeded, opv1alpha1.OperationPhaseFailed, opv1alpha1.OperationPhaseCanceled:
			return false, fmt.Errorf("save %s/%s reached %s at step %q but should have been rejected", got.Namespace, got.Name, got.Status.Phase, got.Status.Step)
		}
		return got.Status.Phase == opv1alpha1.OperationPhaseRejected && !got.Status.TerminatedAt.IsZero(), nil
	})
	if err != nil {
		handleError(t, cs, fx.clusterRef.Name, err)
	}

	assert.Equal(t, opv1alpha1.NotWhitelistedReason, opv1alpha1.RejectedCondition.GetReason(&got.Status))
	assert.True(t, strings.Contains(opv1alpha1.RejectedCondition.GetMessage(&got.Status), "only permits "+opv1alpha1.ETCDSnapshotRestoreResource),
		"the rejection should name what the cluster permits: %q", opv1alpha1.RejectedCondition.GetMessage(&got.Status))
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepPreflight, got.Status.Step, "the save is rejected before it assigns anything")
}

// cancelCertificateRotation sets spec.cancel on the rotation.
func cancelCertificateRotation(t *testing.T, cs *clients.Clients, op *opv1alpha1.CertificateRotation) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := cs.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		latest = latest.DeepCopy()
		latest.Spec.Cancel = true
		_, err = cs.Operation.CertificateRotation().Update(latest)
		return err
	})
	require.NoError(t, err, "cancel op %s/%s", op.Namespace, op.Name)
}

// waitForCertificateRotationTerminated polls until the rotation has terminated, whatever phase it
// ended in, and returns it.
func waitForCertificateRotationTerminated(t *testing.T, cs *clients.Clients, op *opv1alpha1.CertificateRotation) *opv1alpha1.CertificateRotation {
	t.Helper()

	var got *opv1alpha1.CertificateRotation
	err := utilwait.PollUntilContextTimeout(cs.Ctx, 2*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		var err error
		got, err = cs.Operation.CertificateRotation().Get(op.Namespace, op.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return !got.Status.TerminatedAt.IsZero(), nil
	})
	if err != nil {
		handleError(t, cs, op.Spec.ClusterRef.Name, err)
	}
	return got
}
