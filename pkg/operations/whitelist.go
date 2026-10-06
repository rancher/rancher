package operations

import (
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
)

// WhitelistChange is what Adapter.PauseCluster does to the cluster's operation whitelist
// (opv1alpha1.WhitelistedAnnotation), in the same update as the pause.
type WhitelistChange int

const (
	// WhitelistUnchanged leaves the whitelist as it is. The restore uses it to unpause the cluster for
	// its restart while keeping the whitelist until it has succeeded.
	WhitelistUnchanged WhitelistChange = iota

	// WhitelistRestores adds etcd snapshot restores to the whitelist. Operations pause the cluster with
	// it at their point of no return, since past it only a restore can repair the cluster.
	WhitelistRestores

	// WhitelistCleared removes the whitelist. Operations unpause the cluster with it once they have
	// succeeded, which leaves the cluster in a known-good state.
	WhitelistCleared
)

// ApplyWhitelistChange applies change to a cluster's annotations, and returns the annotations to go on
// using (initialized if the change needs to write to a nil map) and whether they changed.
func ApplyWhitelistChange(annotations map[string]string, change WhitelistChange) (map[string]string, bool) {
	switch change {
	case WhitelistRestores:
		value, changed := opv1alpha1.AppendToWhitelist(annotations[opv1alpha1.WhitelistedAnnotation], opv1alpha1.ETCDSnapshotRestoreResource)
		if !changed {
			return annotations, false
		}
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[opv1alpha1.WhitelistedAnnotation] = value
		return annotations, true
	case WhitelistCleared:
		if _, ok := annotations[opv1alpha1.WhitelistedAnnotation]; !ok {
			return annotations, false
		}
		delete(annotations, opv1alpha1.WhitelistedAnnotation)
		return annotations, true
	}
	return annotations, false
}
