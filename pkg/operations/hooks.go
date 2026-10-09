package operations

import (
	"sort"
	"strings"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LifecycleHookDelegate returns the hook identifier and the delegate named by the first label on obj
// whose key begins with prefix and whose value names a delegate, or "", "" when there is none.
//
// An empty prefix returns nothing rather than matching every label: a phase with no hook at all must
// not be reported as having a delegate.
func LifecycleHookDelegate(obj metav1.Object, prefix string) (string, string) {
	hooks := hooksForPrefix(obj, prefix)
	if len(hooks) == 0 {
		return "", ""
	}
	return hooks[0].Id, hooks[0].Delegate
}

type Hook struct {
	Id       string
	Delegate string
}

// hooksForPrefix returns all hooks for an operation, sorted lexicographically by the ID, and then the delegate.
//
// A label whose value is empty is not a hook: it names no delegate, so there is nothing to push onto
// the beacon or to wait on. It is skipped rather than returned, so it can't stand in front of the hooks
// after it, since the first hook is the one taken (see LifecycleHookDelegate).
func hooksForPrefix(obj metav1.Object, prefix string) []Hook {
	if obj == nil || prefix == "" {
		return nil
	}

	hooks := []Hook{}
	for k, v := range obj.GetLabels() {
		if after, ok := strings.CutPrefix(k, prefix); ok {
			if after == "" || v == "" {
				continue
			}
			hooks = append(hooks, Hook{
				Id:       after,
				Delegate: v,
			})
		}
	}

	sort.Slice(hooks, func(i, j int) bool {
		if hooks[i].Id == hooks[j].Id {
			return hooks[i].Delegate < hooks[j].Delegate
		}
		return hooks[i].Id < hooks[j].Id
	})

	return hooks
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

// TerminateAbandoningHooks records terminal handling as complete, abandoning any lifecycle hook the
// operation still carries a label for.
//
// Callers use it where there is no beacon left to hand a delegate — the cluster is gone, or the
// beacon itself is — which is precisely where a hook cannot be satisfied: delegation is a push onto
// the beacon's delegate chain, and there is no chain. Waiting for the label to clear would wait
// forever, and because nothing is terminated in the meantime the operation would also never become
// eligible for TTL collection or be able to retire its finalizer. So the hook is given up on rather
// than waited on.
//
// The label is deliberately left in place: it belongs to whoever set it, it is how they find out
// their hook was reached, and UpdateStatus reports the abandonment on Finalized for as long as it is
// there. See HookAbandonedReason.
func TerminateAbandoningHooks(op metav1.Object, status *opv1alpha1.OperationStatus) {
	// Only the pass that gives up on the hook says so. SetTerminated is idempotent and these paths
	// are reached on every reconcile until the operation is collected, so logging unconditionally
	// would repeat the same line for the whole of the operation's TTL.
	if delegate := TerminalHookDelegate(op, status.Phase); delegate != "" && !IsTerminated(status) {
		logrus.Infof("[operations] %s/%s: abandoning the %s phase hook owed to %q: no beacon remains to delegate it on",
			op.GetNamespace(), op.GetName(), status.Phase, delegate)
	}

	status.SetTerminated()
}

// HasStepHookLabel reports whether obj carries at least one label whose key begins with the given
// step-hook prefix (e.g. "rotate.step.hook.operation.cattle.io/"). Callers use this to detect
// that the operation is in the middle of a step-scoped delegation and thus the operation may not
// currently sit at the top of the beacon's delegate chain: the delegate the step hook pushed is
// there instead. An empty prefix returns false (no label match).
func HasStepHookLabel(obj metav1.Object, stepPrefix string) bool {
	hooks := hooksForPrefix(obj, stepPrefix)
	return len(hooks) > 0
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
