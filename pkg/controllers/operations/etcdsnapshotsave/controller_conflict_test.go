package etcdsnapshotsave

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// An operation still Pending that finds its cluster's beacon held by another operation was created
// alongside it, and both were admitted. The one that acquired the beacon first runs; this one is
// Rejected naming it, having written nothing: not the beacon, and not the cluster. A beacon held by
// anything that isn't an operation is waited on as before.
func TestHandlePending_RejectsAConflictingOperation(t *testing.T) {
	t.Parallel()

	other := ops.BeaconOwnerKey("ETCDSnapshotSave", &metav1.ObjectMeta{Namespace: "fleet-default", Name: "nightly", UID: "nightly-uid"})

	for name, tc := range map[string]struct {
		owner        string
		wantRejected bool
	}{
		"held by another operation": {owner: other, wantRejected: true},
		"held by a handler":         {owner: "some-other-controller"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			beacon := newBeacon(tc.owner, true)
			beacons := &fakeBeaconClient{beacon: beacon}
			adapter := defaultAdapter()
			h := &handler{beacons: beacons}
			s := newScope(newOp(), beacon, adapter)

			got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
			require.NoError(t, err)
			assert.Empty(t, beacons.statusUpdates, "the beacon is another's, so it is left alone")
			assert.Empty(t, beacons.updates)
			assert.Empty(t, adapter.pauseCalls, "nothing on the cluster is touched")

			if !tc.wantRejected {
				assert.NotEqual(t, opv1alpha1.OperationPhaseRejected, got.Phase)
				assert.Equal(t, opv1alpha1.WaitingForBeaconReason, opv1alpha1.PendingCondition.GetReason(&got))
				return
			}
			assert.Equal(t, opv1alpha1.OperationPhaseRejected, got.Phase)
			assert.Equal(t, opv1alpha1.ConflictingOperationReason, opv1alpha1.RejectedCondition.GetReason(&got))
			assert.Contains(t, opv1alpha1.RejectedCondition.GetMessage(&got), "ETCDSnapshotSave fleet-default/nightly")
		})
	}
}
