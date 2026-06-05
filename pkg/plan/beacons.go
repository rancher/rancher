package plan

import (
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
)

// AcquireBeacon acquires a beacon if it is not already owned by the desired owner.
// If the beacon is already owned by the desired owner, it is returned.
// Otherwise, the beacon is updated to be owned by the desired owner and returned.
// If the beacon is owned by another controller, it returns without error, deferring to the caller.
func AcquireBeacon(beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, desiredOwner string) (*planv1alpha1.Beacon, error) {
	if beacon == nil {
		return nil, nil
	} else if beacon.Labels == nil {
		beacon = beacon.DeepCopy()
		beacon.Labels = map[string]string{}
	} else if owner, ok := beacon.Labels[planv1alpha1.BeaconOwnerLabel]; !ok || owner == "" {
		beacon = beacon.DeepCopy()
	} else if owner != desiredOwner {
		return nil, nil
	} else if owner == desiredOwner {
		return beacon, nil
	}

	beacon.Labels[planv1alpha1.BeaconOwnerLabel] = desiredOwner
	return beacons.Update(beacon)
}

// ReleaseBeacon releases a beacon if it is owned by the expected owner.
// If the beacon is not owned by the expected owner, no action is taken.
func ReleaseBeacon(beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, expectedOwner string) error {
	if beacon == nil || beacon.Labels == nil {
		return nil
	}
	if beacon.Labels[planv1alpha1.BeaconOwnerLabel] == expectedOwner {
		beacon = beacon.DeepCopy()
		delete(beacon.Labels, planv1alpha1.BeaconOwnerLabel)
		_, err := beacons.Update(beacon)
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

// HoldingBeacon returns true if the beacon is owned by the desired owner.
func HoldingBeacon(beacon *planv1alpha1.Beacon, desiredOwner string) bool {
	if beacon == nil || beacon.Labels == nil {
		return false
	}
	return beacon.Labels[planv1alpha1.BeaconOwnerLabel] == desiredOwner
}
