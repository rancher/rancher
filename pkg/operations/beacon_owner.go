package operations

import (
	"strings"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BeaconOwnerKeyPrefix opens the key namespace operations claim beacons under. A beacon's owner is
// an opaque string and the holder need not be an object at all (a handler which operates on a
// cluster directly holds it under its own name, a lifecycle hook delegate holds it under the name
// its label carries), so this prefix is what lets operations recognize their own claims and leave
// everybody else's alone.
const BeaconOwnerKeyPrefix = "operation.cattle.io"

// beaconOwnerKeyParts is the number of "/"-separated segments in an operation's claim:
// prefix, kind, namespace, name, uid.
const beaconOwnerKeyParts = 5

// BeaconOwner is the operation named by a beacon claim written with BeaconOwnerKey.
type BeaconOwner struct {
	Kind      string
	Namespace string
	Name      string
	UID       string
}

// BeaconOwnerKey renders the beacon claim for an operation:
//
//	operation.cattle.io/<Kind>/<namespace>/<name>/<uid>
//
// The UID is what makes a claim belong to one particular object rather than to a name. Without it a
// claim left behind by an operation which was deleted mid-flight would be silently adopted by the
// next operation created with the same name, inheriting whatever the dead one left on the beacon;
// with it, that claim cannot be mistaken for the new operation's own (see SupersedesBeaconOwner).
//
// It is also the identity the operation's plans already carry, since OperationEnv stamps the same
// UID into every plan it assigns: so a claim on the beacon and the plan content written under it
// name the same object.
func BeaconOwnerKey(kind string, op metav1.Object) string {
	if op == nil {
		return ""
	}

	return strings.Join([]string{
		BeaconOwnerKeyPrefix,
		kind,
		op.GetNamespace(),
		op.GetName(),
		string(op.GetUID()),
	}, "/")
}

// ParseBeaconOwner reads a claim written by BeaconOwnerKey. It reports false for any claim that is
// not an operation's, which callers must treat as "not mine, and none of my business": a beacon held
// by a handler, by a hook delegate, or by the imported-day2ops-disable key is being held for reasons
// this package cannot see, and must be left exactly as it is.
//
// Every segment is required. A claim missing one is not one this package wrote, so it is reported as
// unparseable rather than partially trusted: the whole point of the key is to identify one object,
// and a claim which cannot do that must never be taken for a live operation's nor cleared as a dead
// one's.
func ParseBeaconOwner(key string) (BeaconOwner, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != beaconOwnerKeyParts || parts[0] != BeaconOwnerKeyPrefix {
		return BeaconOwner{}, false
	}

	owner := BeaconOwner{Kind: parts[1], Namespace: parts[2], Name: parts[3], UID: parts[4]}
	if owner.Kind == "" || owner.Namespace == "" || owner.Name == "" || owner.UID == "" {
		return BeaconOwner{}, false
	}

	return owner, true
}

// SupersedesBeaconOwner reports whether the claim recorded on a beacon was left by an earlier
// incarnation of the operation now claiming it: the same kind, namespace and name, but a different
// object.
//
// That makes the recorded claimant provably gone, with nothing to look up. Two objects cannot hold
// one name in one namespace at the same time, so an operation which finds its own name on a beacon
// under a different UID is looking at the remains of an object that no longer exists: whether it
// was deleted without its finalizer running, or rolled back out of existence by an etcd restore of
// the cluster it lived in. Either way the claim is dead and the operation now holding the name is
// entitled to clear it.
//
// The rule is deliberately limited to the operation's own name. Proving that some *other*
// operation's claim is dead needs its object looked up, which is a different mechanism.
func SupersedesBeaconOwner(mine, recorded string) bool {
	claiming, ok := ParseBeaconOwner(mine)
	if !ok {
		return false
	}

	held, ok := ParseBeaconOwner(recorded)
	if !ok {
		return false
	}

	return claiming.Kind == held.Kind &&
		claiming.Namespace == held.Namespace &&
		claiming.Name == held.Name &&
		claiming.UID != held.UID
}

// ReclaimSupersededBeacon clears a beacon whose claim was left behind by an earlier incarnation of
// ownerKey's operation, so the operation holding that name now can acquire it cleanly. Any other
// claim (a live operation's, another operation's, or a holder which is not an operation at all)
// is left untouched, and the caller finds the beacon unavailable as it would have anyway.
//
// The claim is cleared in full rather than just handed over. The delegates on a dead operation's
// beacon were pushed on its behalf to gate its lifecycle hooks, and those hooks went with the object
// they were labeled on, so nothing will ever pop them; leaving them would hand the new operation a
// beacon it is not authorized to drive. Active goes the same way: it described the dead operation's
// work.
func ReclaimSupersededBeacon(beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, ownerKey string) (*planv1alpha1.Beacon, error) {
	if beacon == nil {
		return beacon, nil
	}

	recorded := beacon.Status.Owner
	if recorded == "" || recorded == ownerKey || !SupersedesBeaconOwner(ownerKey, recorded) {
		return beacon, nil
	}

	logrus.Infof("[operations] reclaiming beacon %s/%s from superseded claim %q on behalf of %q",
		beacon.Namespace, beacon.Name, recorded, ownerKey)

	beacon = beacon.DeepCopy()
	beacon.Status.Active = false
	beacon.Status.Owner = ""
	beacon.Status.Delegates = nil

	return beacons.UpdateStatus(beacon)
}
