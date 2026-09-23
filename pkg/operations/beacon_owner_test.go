package operations

import (
	"testing"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func op(namespace, name, uid string) metav1.Object {
	return &metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid)}
}

func TestBeaconOwnerKeyRoundTrip(t *testing.T) {
	t.Parallel()

	key := BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "nightly", "9f2c"))
	assert.Equal(t, "operation.cattle.io/ETCDSnapshotSave/fleet-default/nightly/9f2c", key)

	owner, ok := ParseBeaconOwner(key)
	assert.True(t, ok)
	assert.Equal(t, BeaconOwner{Kind: "ETCDSnapshotSave", Namespace: "fleet-default", Name: "nightly", UID: "9f2c"}, owner)
}

// TestParseBeaconOwnerRejectsForeignClaims is the load-bearing half of the format. A beacon's owner
// is an opaque string and the holder need not be an object at all, so anything this package did not
// write has to come back unparseable — reported neither as a live operation's claim nor cleared as
// a dead one's.
func TestParseBeaconOwnerRejectsForeignClaims(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"",
		// A handler holding the beacon under its own name, and the key that blocks day-2 ops
		// on an imported cluster.
		"etcd-snapshot-save",
		"imported-day2ops-disable",
		// A lifecycle hook delegate, named by whatever its label carried.
		"my-cleanup-hook",
		// The name-based scheme this format replaced, which carries no identity.
		"etcd-snapshot-save/fleet-default/nightly",
		// Right shape, wrong namespace of keys.
		"other.cattle.io/ETCDSnapshotSave/fleet-default/nightly/9f2c",
		// Too many segments: a name or uid containing a separator would alias otherwise.
		"operation.cattle.io/ETCDSnapshotSave/fleet-default/nightly/9f2c/extra",
		// Missing segments.
		"operation.cattle.io/ETCDSnapshotSave/fleet-default/nightly/",
		"operation.cattle.io/ETCDSnapshotSave/fleet-default//9f2c",
		"operation.cattle.io//fleet-default/nightly/9f2c",
	} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			_, ok := ParseBeaconOwner(key)
			assert.False(t, ok, "%q must not be read as an operation's claim", key)
		})
	}
}

// TestSupersedesBeaconOwner covers the rule that needs no lookup: two objects cannot hold one name
// in one namespace at the same time, so an operation which finds its own name on a beacon under a
// different uid is looking at the remains of an object that no longer exists.
func TestSupersedesBeaconOwner(t *testing.T) {
	t.Parallel()

	mine := BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "nightly", "new-uid"))

	cases := []struct {
		name     string
		recorded string
		want     bool
	}{
		{
			name:     "same name, earlier object",
			recorded: BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "nightly", "old-uid")),
			want:     true,
		},
		{
			name:     "our own claim is not superseded",
			recorded: mine,
		},
		{
			name:     "same name in another namespace",
			recorded: BeaconOwnerKey("ETCDSnapshotSave", op("fleet-local", "nightly", "old-uid")),
		},
		{
			// Only the operation's own name proves anything. Another operation may well be alive.
			name:     "another operation of the same kind",
			recorded: BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "weekly", "old-uid")),
		},
		{
			// Names are unique per kind, not across kinds.
			name:     "same name, another kind",
			recorded: BeaconOwnerKey("ETCDSnapshotRestore", op("fleet-default", "nightly", "old-uid")),
		},
		{
			name:     "a holder which is not an operation",
			recorded: "imported-day2ops-disable",
		},
		{
			name:     "unclaimed",
			recorded: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, SupersedesBeaconOwner(mine, tc.recorded))
		})
	}

	assert.False(t, SupersedesBeaconOwner("etcd-snapshot-save", mine),
		"a claimant which is not an operation supersedes nothing")
}

// fakeBeaconClient records the status updates ReclaimSupersededBeacon makes. Beacon ownership lives
// in the status, which has its own subresource, so the reclaim has to go through UpdateStatus.
type fakeBeaconClient struct {
	plancontrollers.BeaconClient

	statusUpdates []*planv1alpha1.Beacon
	updates       []*planv1alpha1.Beacon
}

func (f *fakeBeaconClient) Update(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.updates = append(f.updates, b.DeepCopy())
	return b, nil
}

func (f *fakeBeaconClient) UpdateStatus(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, b.DeepCopy())
	return b, nil
}

func TestReclaimSupersededBeacon(t *testing.T) {
	t.Parallel()

	mine := BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "nightly", "new-uid"))
	superseded := BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "nightly", "old-uid"))

	held := func(owner string) *planv1alpha1.Beacon {
		return &planv1alpha1.Beacon{
			ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "test"},
			Status: planv1alpha1.BeaconStatus{
				Owner:     owner,
				Active:    true,
				Delegates: []string{"delegate-a"},
			},
		}
	}

	t.Run("clears a superseded claim in full", func(t *testing.T) {
		t.Parallel()

		beacons := &fakeBeaconClient{}
		got, err := ReclaimSupersededBeacon(held(superseded), beacons, mine)
		assert.NoError(t, err)

		if assert.Len(t, beacons.statusUpdates, 1, "the claim must be cleared through the status subresource") {
			assert.Equal(t, "", got.Status.Owner)
			// The delegates were pushed to gate hooks that died with the object, so nothing will
			// ever pop them; handing them on would leave the new operation unable to drive its own
			// beacon.
			assert.Empty(t, got.Status.Delegates)
			assert.False(t, got.Status.Active)
		}
		assert.Empty(t, beacons.updates, "ownership is status, not spec")
	})

	for _, tc := range []struct {
		name  string
		owner string
	}{
		{name: "leaves our own claim", owner: mine},
		{name: "leaves an unclaimed beacon", owner: ""},
		{name: "leaves another operation's claim", owner: BeaconOwnerKey("ETCDSnapshotSave", op("fleet-default", "weekly", "other-uid"))},
		{name: "leaves a holder which is not an operation", owner: "imported-day2ops-disable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			beacons := &fakeBeaconClient{}
			got, err := ReclaimSupersededBeacon(held(tc.owner), beacons, mine)
			assert.NoError(t, err)
			assert.Equal(t, tc.owner, got.Status.Owner, "the claim must be left exactly as it was")
			assert.Empty(t, beacons.statusUpdates)
			assert.Empty(t, beacons.updates)
		})
	}

	t.Run("tolerates a missing beacon", func(t *testing.T) {
		t.Parallel()

		beacons := &fakeBeaconClient{}
		got, err := ReclaimSupersededBeacon(nil, beacons, mine)
		assert.NoError(t, err)
		assert.Nil(t, got)
		assert.Empty(t, beacons.statusUpdates)
	})
}
