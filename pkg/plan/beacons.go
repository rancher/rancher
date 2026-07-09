package plan

import (
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AcquireBeacon acquires a beacon if it is not already owned by the desired owner.
// If the beacon is already owned by the desired owner, it is returned.
// Otherwise, the beacon is updated to be owned by the desired owner and returned.
// If the beacon is owned by another controller, it returns without error, deferring to the caller.
// During the pending phase, the beacon may not necessarily exist yet, and the webhook should prevent duplicate
// operations from being created.
// The InProgressPhase should instead ensure the beacon is still held and return an error as a result.
func AcquireBeacon(beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, desired string) (*planv1alpha1.Beacon, error) {
	if beacon == nil {
		return nil, nil
	} else if AuthorizedForBeacon(beacon, desired) {
		return beacon, nil
	} else if owner := beacon.Status.Owner; owner == "" {
		beacon = beacon.DeepCopy()
	} else if owner != desired {
		return nil, nil
	} else if owner == desired {
		return beacon, nil
	}

	beacon.Status.Owner = desired

	return beacons.UpdateStatus(beacon)
}

// ReleaseBeacon releases a beacon on behalf of `expected`:
//   - If `expected` is the primary owner, the beacon is fully torn down (Active=false, Owner="",
//     Delegates=nil).
//   - Otherwise, if `expected` appears anywhere in the delegate chain, it is removed from the
//     chain. This is broader than PopDelegate (which only pops the top) because a terminating
//     operation may hold a mid-chain slot — its dependent delegates should already have popped
//     themselves off by the time we run cleanup, but if they haven't we still need to prevent
//     leaking our own reference.
//   - If `expected` is neither the owner nor in the chain, no action is taken.
func ReleaseBeacon(beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, expected string) error {
	if beacon == nil {
		return nil
	}

	if beacon.Status.Owner == expected {
		beacon = beacon.DeepCopy()

		beacon.Status.Active = false
		beacon.Status.Owner = ""
		beacon.Status.Delegates = nil

		_, err := beacons.UpdateStatus(beacon)
		return err
	}

	// Remove every occurrence of `expected` from the delegate chain. Under normal operation
	// there is at most one, but the loop is cheap and covers the pathological case where a
	// controller pushed itself onto the chain twice.
	removed := false
	filtered := beacon.Status.Delegates[:0:0]
	for _, delegate := range beacon.Status.Delegates {
		if delegate == expected {
			removed = true
			continue
		}
		filtered = append(filtered, delegate)
	}
	if removed {
		beacon = beacon.DeepCopy()
		beacon.Status.Delegates = filtered
		_, err := beacons.UpdateStatus(beacon)
		return err
	}

	return nil
}

// ToggleBeacon toggles the active status of a beacon.
// If the beacon is already in the desired state, no action is taken.
func ToggleBeacon(beacon *planv1alpha1.Beacon, active bool, beacons plancontrollers.BeaconClient) (*planv1alpha1.Beacon, error) {
	if beacon.Status.Active != active {
		beacon = beacon.DeepCopy()
		beacon.Status.Active = active
		beacon, err := beacons.UpdateStatus(beacon)
		return beacon, err
	}

	return beacon, nil
}

// AuthorizedForBeacon returns true if the beacon is owned by the desired owner.
// If desired is empty string, validates nothing is holding beacon.
func AuthorizedForBeacon(beacon *planv1alpha1.Beacon, desired string) bool {
	if IsDelegateBeaconHolder(beacon, desired) {
		return true
	}

	return IsOwningBeaconHolder(beacon, desired)
}

func IsOwningBeaconHolder(beacon *planv1alpha1.Beacon, desired string) bool {
	if beacon == nil {
		return desired == ""
	}

	return beacon.Status.Owner == desired
}

func IsActiveBeaconHolder(beacon *planv1alpha1.Beacon, desired string) bool {
	if beacon == nil {
		return false
	}

	if beacon.Status.Owner == desired {
		return true
	}

	if len(beacon.Status.Delegates) > 0 {
		if beacon.Status.Delegates[len(beacon.Status.Delegates)-1] == desired {
			return true
		}
	}

	return false
}

func IsDelegateBeaconHolder(beacon *planv1alpha1.Beacon, desired string) bool {
	if desired == "" {
		return false
	}

	if beacon == nil || len(beacon.Status.Delegates) == 0 {
		return false
	}

	return beacon.Status.Delegates[len(beacon.Status.Delegates)-1] == desired
}

func IsInDelegateChain(beacon *planv1alpha1.Beacon, desired string) bool {
	if desired == "" {
		return false
	}

	if beacon == nil || len(beacon.Status.Delegates) == 0 {
		return false
	}

	for _, delegate := range beacon.Status.Delegates {
		if delegate == desired {
			return true
		}
	}

	return false
}

func PushDelegate(beacon *planv1alpha1.Beacon, delegate string, beacons plancontrollers.BeaconClient) (*planv1alpha1.Beacon, error) {
	if beacon == nil {
		return beacon, nil
	}

	if len(beacon.Status.Delegates) > 0 {
		if beacon.Status.Delegates[len(beacon.Status.Delegates)-1] == delegate {
			// already delegated
			return beacon, nil
		}
	}

	beacon = beacon.DeepCopy()

	if beacon.Status.Delegates == nil {
		beacon.Status.Delegates = []string{}
	}

	beacon.Status.Delegates = append(beacon.Status.Delegates, delegate)
	beacon, err := beacons.UpdateStatus(beacon)
	return beacon, err
}

func PopDelegate(beacon *planv1alpha1.Beacon, delegate string, beacons plancontrollers.BeaconClient) (*planv1alpha1.Beacon, error) {
	if beacon == nil {
		return beacon, nil
	}

	if len(beacon.Status.Delegates) == 0 {
		return beacon, nil
	}

	current := beacon.Status.Delegates[len(beacon.Status.Delegates)-1]
	if current != delegate {
		return beacon, nil
	}

	beacon = beacon.DeepCopy()

	beacon.Status.Delegates = beacon.Status.Delegates[:len(beacon.Status.Delegates)-1]
	beacon, err := beacons.UpdateStatus(beacon)
	return beacon, err
}

func ControllerOwnerKey(obj metav1.Object, prefix string) string {
	if obj == nil {
		return ""
	}

	key := obj.GetName()
	if namespace := obj.GetNamespace(); namespace != "" {
		key = namespace + "/" + key
	}

	return prefix + "/" + key
}
