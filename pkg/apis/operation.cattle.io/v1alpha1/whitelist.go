package v1alpha1

import "strings"

const (
	// WhitelistedAnnotation restricts which operations may be created for a cluster. It is set on the
	// object an operation's spec.clusterRef names, and its value is a comma-separated list of the
	// operation resources that may still be created, each named the way its CustomResourceDefinition
	// is, "<plural>.<group>".
	//
	// An absent or empty annotation restricts nothing. An operation adds ETCDSnapshotRestoreResource
	// when it takes the cluster past its point of no return (when it pauses the cluster), since only a
	// restore can repair a cluster an operation was stopped part-way through, and removes the
	// annotation when it succeeds.
	WhitelistedAnnotation = "operation.cattle.io/whitelisted"

	// ETCDSnapshotRestoreResource names ETCDSnapshotRestore in WhitelistedAnnotation.
	ETCDSnapshotRestoreResource = "etcdsnapshotrestores.operation.cattle.io"
)

// WhitelistEntries returns the entries of a WhitelistedAnnotation value: its comma-separated items,
// with surrounding space trimmed and empty items dropped.
func WhitelistEntries(value string) []string {
	var entries []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			entries = append(entries, item)
		}
	}
	return entries
}

// HasWhitelist reports whether annotations restrict which operations may be created for the cluster
// they belong to: WhitelistedAnnotation is present with at least one entry.
func HasWhitelist(annotations map[string]string) bool {
	return len(WhitelistEntries(annotations[WhitelistedAnnotation])) > 0
}

// Whitelisted reports whether an operation of the given resource ("<plural>.<group>") may be created
// for the cluster carrying annotations: the cluster has no whitelist, or the resource is in it.
func Whitelisted(annotations map[string]string, resource string) bool {
	entries := WhitelistEntries(annotations[WhitelistedAnnotation])
	if len(entries) == 0 {
		return true
	}
	for _, entry := range entries {
		if entry == resource {
			return true
		}
	}
	return false
}

// AppendToWhitelist returns the WhitelistedAnnotation value with resource added to it, and whether
// that changed it. An absent or empty value becomes just the resource; an entry already present is
// not added again. Entries are written back trimmed and comma-separated.
func AppendToWhitelist(value, resource string) (string, bool) {
	entries := WhitelistEntries(value)
	for _, entry := range entries {
		if entry == resource {
			return value, false
		}
	}
	return strings.Join(append(entries, resource), ","), true
}
