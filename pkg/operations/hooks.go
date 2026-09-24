package operations

import (
	"strings"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasActiveLifecycleHook reports whether obj still carries at least one lifecycle-hook label
// (phase or step). While such a label is present, the op's owning controller MUST NOT garbage
// collect the object even after its terminal phase and TTL have expired: the delegate needs a
// chance to observe the current phase and pop itself from the beacon's delegate chain, and it
// signals it is done by removing the label.
//
// Recognizes any label key containing opv1alpha1.LifecycleHookLabelMarker. That catches every phase
// prefix and every step-level prefix declared by an operation controller package, so callers do not
// have to enumerate them.
func HasActiveLifecycleHook(obj metav1.Object) bool {
	if obj == nil {
		return false
	}
	for k := range obj.GetLabels() {
		if strings.Contains(k, opv1alpha1.LifecycleHookLabelMarker) {
			return true
		}
	}
	return false
}

// LifecycleHookDelegate returns the hook identifier and the delegate named by the first label on obj
// whose key begins with prefix, or "", "" when there is none.
//
// An empty prefix returns nothing rather than matching every label: a phase with no hook at all must
// not be reported as having a delegate.
func LifecycleHookDelegate(obj metav1.Object, prefix string) (string, string) {
	if obj == nil || prefix == "" {
		return "", ""
	}

	for k, v := range obj.GetLabels() {
		if after, ok := strings.CutPrefix(k, prefix); ok {
			return after, v
		}
	}

	return "", ""
}

// TerminalHookDelegate returns the delegate the operation's terminal phase hook is still waiting on,
// or "" when none is owed.
//
// A terminal operation is not finished with while that label is present: the delegate has not had
// its turn on the beacon, so neither the controller nor anything reading Finalized may treat the
// operation as wrapped up. A non-terminal phase has no terminal hook and so owes nothing.
func TerminalHookDelegate(op metav1.Object, phase opv1alpha1.OperationPhase) string {
	_, delegate := LifecycleHookDelegate(op, TerminalPhaseHookPrefix(phase))

	return delegate
}

// TerminateUnlessHookOwed records terminal handling as complete unless the operation's terminal
// phase hook is still owed a turn. Callers use it where there is nothing left to hand a delegate —
// no cluster, or no beacon — so the operation would otherwise be recorded as wrapped up while its
// delegate had never been given the beacon it was promised.
//
// A deleting operation is the exception: it is discarded along with its hooks, so it terminates at
// once rather than holding its finalizer open for a delegate that will never be handed anything.
func TerminateUnlessHookOwed(op metav1.Object, status *opv1alpha1.OperationStatus) {
	if op.GetDeletionTimestamp() != nil || TerminalHookDelegate(op, status.Phase) == "" {
		status.SetTerminated()
	}
}

// HasStepHookLabel reports whether obj carries at least one label whose key begins with the given
// step-hook prefix (e.g. "rotate.step.hook.operation.cattle.io/"). Callers use this to detect
// that the operation is in the middle of a step-scoped delegation and thus the operation may not
// currently sit at the top of the beacon's delegate chain: the delegate the step hook pushed is
// there instead. An empty prefix returns false (no label match).
func HasStepHookLabel(obj metav1.Object, stepPrefix string) bool {
	if obj == nil || stepPrefix == "" {
		return false
	}
	for k := range obj.GetLabels() {
		if strings.HasPrefix(k, stepPrefix) {
			return true
		}
	}
	return false
}

// DelegateForHook pushes the delegate named by obj's lifecycle-hook label for prefix onto the
// beacon's delegate chain, and reports whether there was one to push. A hook that has already been
// delegated is reported without touching the beacon again, so the caller can keep calling this for
// as long as the label is present.
//
// This is where lifecycle hooks meet the beacon, and it lives here rather than with the beacon
// primitives it is built from because hooks are an operations paradigm: pkg/plan knows how to push
// a delegate onto a chain, and knows nothing about the labels that decide when to.
//
// The returned beacon is always usable, the one passed in when nothing changed or the push failed,
// the updated one otherwise, so a caller can assign it back unconditionally.
func DelegateForHook(obj metav1.Object, beacon *planv1alpha1.Beacon, beacons plancontrollers.BeaconClient, prefix string) (bool, *planv1alpha1.Beacon, error) {
	_, delegate := LifecycleHookDelegate(obj, prefix)
	if delegate == "" {
		return false, beacon, nil
	}

	if planapi.IsInDelegateChain(beacon, delegate) {
		return true, beacon, nil
	}

	updated, err := planapi.PushDelegate(beacon, delegate, beacons)
	if err != nil {
		return true, beacon, err
	}

	return true, updated, nil
}
