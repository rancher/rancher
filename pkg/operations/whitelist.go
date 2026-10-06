package operations

import (
	"fmt"
	"strings"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

	// WhitelistKeepsPause leaves the whitelist as it is, and makes an unpause leave a whitelisted
	// cluster as it is. A rejected operation unpauses the cluster with it: being rejected before its
	// point of no return, it has nothing of its own to undo, and a cluster that carries a whitelist
	// was paused by whichever operation added it, and has to stay that way until a restore repairs it.
	WhitelistKeepsPause
)

// heldByWhitelist reports whether the cluster carrying annotations is to be left exactly as it is
// rather than given the requested pause: an unpause with WhitelistKeepsPause, of a whitelisted cluster.
func heldByWhitelist(annotations map[string]string, pause bool, change WhitelistChange) bool {
	return !pause && change == WhitelistKeepsPause && opv1alpha1.HasWhitelist(annotations)
}

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

// WhitelistProblem returns why an operation of the given resource ("<plural>.<group>") may not run on
// the cluster clusterRef names, whose object carries annotations, or "" if it may.
//
// The operation webhook turns away a create the cluster's whitelist doesn't allow, but it reads a
// cache, so an operation created just as an earlier one stopped part-way through can still be
// admitted. Every operation checks again in its preflight, holding the beacon, before it changes
// anything: only the beacon's holder adds a whitelist, so the answer doesn't change under it. That
// is also what lets a rotation remove the whitelist when it succeeds, since it only ever finds one it
// added itself.
func WhitelistProblem(clusterRef *corev1.ObjectReference, annotations map[string]string, resource string) string {
	if opv1alpha1.Whitelisted(annotations, resource) {
		return ""
	}

	cluster := "the cluster"
	if clusterRef != nil {
		name := clusterRef.Name
		if clusterRef.Namespace != "" {
			name = clusterRef.Namespace + "/" + name
		}
		cluster = fmt.Sprintf("%s %s", clusterRef.Kind, name)
	}
	return fmt.Sprintf("%s only permits %s: an earlier operation was stopped after pausing it, and the cluster requires an etcd snapshot restore",
		cluster, strings.Join(opv1alpha1.WhitelistEntries(annotations[opv1alpha1.WhitelistedAnnotation]), ", "))
}
