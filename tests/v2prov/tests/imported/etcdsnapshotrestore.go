package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// waitForBackpopulatedSnapshot polls until at least one ETCDSnapshot CR has been back-populated for
// the imported cluster, then returns the most recently created one. The snapshotbackpopulate
// controller writes these CRs into the namespace whose name matches the cluster (cluster-scoped
// mgmt clusters use namespace == name).
func waitForBackpopulatedSnapshot(t *testing.T, clients *clients.Clients, clusterName string, createdAfter time.Time) *rkev1.ETCDSnapshot {
	t.Helper()

	var picked *rkev1.ETCDSnapshot
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 5*time.Minute, true, func(_ context.Context) (bool, error) {
		list, err := clients.RKE.ETCDSnapshot().List(clusterName, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, clusterName),
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

// RunETCDSnapshotRestoreOperationTest creates an ETCDSnapshotRestore operation targeting the given
// clusterRef and waits for it to reach the Succeeded phase. Mirrors RunETCDSnapshotSaveOperationTest
// — exercises the operation.cattle.io/v1alpha1 ETCDSnapshotRestore controller end-to-end.
func RunETCDSnapshotRestoreOperationTest(t *testing.T, clients *clients.Clients, namespace, snapshotName string, clusterRef corev1.ObjectReference) *opv1alpha1.ETCDSnapshotRestore {
	t.Helper()

	op := &opv1alpha1.ETCDSnapshotRestore{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-restore-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.ETCDSnapshotRestoreSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
			},
			Args: opv1alpha1.ETCDSnapshotRestoreArgs{
				Name: snapshotName,
			},
		},
	}

	op, err := clients.Operation.ETCDSnapshotRestore().Create(op)
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
