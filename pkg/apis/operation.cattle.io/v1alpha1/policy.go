package v1alpha1

import (
	"strconv"
	"strings"
)

const (
	// PreemptionPriorityAnnotation is set on an operation's CustomResourceDefinition and declares how
	// that kind of operation participates in preemption. A new operation may only be created while
	// other operations are in flight if its priority is greater than zero and greater than or equal
	// to the priority of every in-flight operation, in which case those operations are canceled.
	//
	// The value is a plain integer. An absent or unparseable value is treated as zero, which is the
	// most restrictive setting: such an operation may never preempt, and may therefore never be
	// created while any other operation is in flight.
	PreemptionPriorityAnnotation = "operation.cattle.io/preemption-priority"

	// RestoreRequiredOnCancellationAnnotation is set on an operation's CustomResourceDefinition and
	// declares that cancelling that kind of operation may leave the cluster in a state which can only
	// be recovered by restoring it. When such an operation is canceled, its cluster is annotated with
	// RestoreRequiredAnnotation.
	RestoreRequiredOnCancellationAnnotation = "operation.cattle.io/restore-required-on-cancellation"

	// ClearsRestoreRequiredAnnotation is set on an operation's CustomResourceDefinition and declares
	// that successful completion of that kind of operation returns the cluster to a known-good state.
	// When such an operation succeeds, RestoreRequiredAnnotation is removed from its cluster.
	ClearsRestoreRequiredAnnotation = "operation.cattle.io/clears-restore-required"

	// RestoreRequiredAnnotation is set on a cluster object to record that an operation which mutates
	// cluster state was canceled part-way through, and that the cluster should be restored. The value
	// identifies the operation which set it, as "<Kind>/<namespace>/<name>".
	RestoreRequiredAnnotation = "operation.cattle.io/restore-required"

	// CanceledByAnnotation is set on an operation which was canceled by another operation preempting
	// it, rather than by a user. The value identifies the preempting operation, as
	// "<Kind>/<namespace>/<name>".
	CanceledByAnnotation = "operation.cattle.io/canceled-by"
)

// PreemptionPriority returns the preemption priority declared by the given annotations, which are
// expected to be those of an operation's CustomResourceDefinition. An absent or unparseable value
// yields zero.
func PreemptionPriority(annotations map[string]string) int {
	priority, err := strconv.Atoi(strings.TrimSpace(annotations[PreemptionPriorityAnnotation]))
	if err != nil {
		return 0
	}

	return priority
}

// CanPreempt returns true if an operation with priority mine may cancel an in-flight operation with
// priority theirs. Operations without a positive priority never preempt, so by default the presence
// of any in-flight operation blocks the creation of another.
func CanPreempt(mine, theirs int) bool {
	return mine > 0 && mine >= theirs
}

// BoolAnnotation returns true if the annotation with the given key is present and parses as true.
func BoolAnnotation(annotations map[string]string, key string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(annotations[key]))
	if err != nil {
		return false
	}

	return value
}

// ObjectRefKey formats a kind, namespace, and name as the value used by RestoreRequiredAnnotation and
// CanceledByAnnotation.
func ObjectRefKey(kind, namespace, name string) string {
	if namespace == "" {
		return kind + "/" + name
	}

	return kind + "/" + namespace + "/" + name
}
