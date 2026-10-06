package operations

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PlanDispatchedBy reports whether the plan currently assigned to secret was last written by the
// operation holding the beacon under ownerKey, its BeaconOwnerKey.
//
// planapi.PlanWriterAnnotation is the record of that: every write the Store makes to a plan names
// the beacon holder it was made for. A secret which has since been reassigned by another operation,
// or written to by a lifecycle hook delegate, names that writer instead and is not claimed here,
// which is what makes acting on this safe for an operation that no longer holds the beacon.
//
// A secret with no plan belongs to nobody: there is nothing this operation could usefully do to it.
func PlanDispatchedBy(secret *corev1.Secret, ownerKey string) bool {
	if secret == nil || ownerKey == "" || len(secret.Data[planapi.PlanDataKey]) == 0 {
		return false
	}

	return secret.Annotations[planapi.PlanWriterAnnotation] == ownerKey
}

// DispatchedPlanStopTimeout bounds how long an operation that ended with plans still running holds
// the beacon waiting for the agents to report that they have stopped. An agent that is down, or a
// node that is gone, never reports anything, and waiting on it without a bound would hold the
// cluster's beacon until the operation was deleted, and deleting it would only arrive back here.
const DispatchedPlanStopTimeout = 5 * time.Minute

// stopClock is replaced in tests.
var stopClock = time.Now

// StopDispatchedPlans asks the agents to abort every plan the operation holding the beacon under
// ownerKey dispatched to the given cluster and has not yet seen finish, and reports whether they all
// have. A terminal phase handler calls it before releasing the beacon, and keeps the beacon until it
// reports true.
//
// This is what makes an operation's terminal outcome mean something on the cluster rather than only
// in its status. Marking the operation Canceled or Failed stops the controller dispatching anything
// further, but a plan already sitting in a machine-plan secret is the agent's to run, and it will
// keep running it — so an operation that released the beacon on the strength of its status alone
// would let the next operation start while the previous one's instructions were still executing.
// Canceling the plans, and waiting for the agents to report that they have stopped, is what closes
// that window. See planapi.PlanCanceledAnnotation.
//
// Only plans the agent may still act on are canceled and waited on (see planapi.PlanState.IsActive).
// One that has finished has nothing left to stop: a succeeded plan's one-time instructions have all
// run, and canceling it would only stop its periodic instructions and record a cancellation of work
// which in fact completed. A canceled plan has stopped once the agent records it as canceled, which
// it does only after the instruction it interrupted has been terminated; one that finished in its
// own right before the agent saw the cancellation is just as stopped.
//
// It reports true without waiting for everything, once waiting no longer protects anything:
//   - the operation has no claim on the beacon, so releasing it is a no-op and whoever holds the
//     beacon now is not this operation's to protect;
//   - a plan was reassigned by another writer, or its secret is gone, which leaves it out of the
//     plans waited on;
//   - DispatchedPlanStopTimeout has passed since the operation reached its terminal phase.
//
// While it waits, it reports so on the Finalized condition. Once it is done, it notes on the
// outcome condition anything an operator has to check before starting another disruptive
// operation: the plans the agents never reported stopping, and those whose agents reported
// planapi.PlanCheckpoint.TerminationIncomplete, meaning work the plan started may still be running.
//
// Only plans this operation wrote are touched, so a secret another operation has since taken over is
// left to it. And nothing is written at all unless the operation is still the beacon's current
// holder: a write on behalf of an operation that has lost the beacon would be reaching into the work
// of whoever holds it now, and the machine-plan webhook rejects one which slips through between this
// check and the write. "Current holder" is planapi.AuthorizedForBeacon: the beacon's owner, or the
// delegate it was last handed to. An operation part-way down the delegate chain has handed its
// authority on without giving the beacon up, so this returns an error for the caller to try again
// once it is handed back.
//
// There is nothing to cancel without a cluster to enumerate secrets under or a store to write
// through, so it reports true rather than failing: an operation whose cluster has gone has no agent
// left to run anything it dispatched either.
func StopDispatchedPlans(store *planapi.Store, secrets planapi.SecretClient, cluster *unstructured.Unstructured, namespace string,
	op metav1.Object, ownerKey string, beacon *planv1alpha1.Beacon, status *opv1alpha1.OperationStatus,
) (bool, error) {
	if store == nil || secrets == nil || cluster == nil || op == nil || ownerKey == "" {
		return true, nil
	}

	if !planapi.HoldsBeacon(beacon, ownerKey) {
		logrus.Infof("[operations] %s/%s: no longer holds the beacon, leaving the plans it dispatched to whoever holds it now",
			op.GetNamespace(), op.GetName())

		return true, nil
	}
	if !planapi.AuthorizedForBeacon(beacon, ownerKey) {
		return false, fmt.Errorf("cannot cancel the plans %s/%s dispatched while it has handed the beacon on to a delegate",
			op.GetNamespace(), op.GetName())
	}

	collected, err := planapi.NewCollector(secrets, cluster, namespace).Collect()
	if planapi.IsTransient(err) {
		return false, err
	} else if err != nil {
		// The machine-plan secrets cannot be enumerated at all and no retry will change that, so
		// this is reported and stepped over rather than returned. The operation still has to be able
		// to finish: refusing to would hold the beacon forever, freezing the cluster for every
		// operation behind it, with no way out but deleting this one — which arrives back here and
		// fails in exactly the same way.
		logrus.Errorf("[operations] %s/%s: cannot enumerate machine-plan secrets to cancel the plans it dispatched, any still in flight will run to completion: %v",
			op.GetNamespace(), op.GetName(), err)

		return true, nil
	}

	var running, terminationIncomplete []string

	for _, secret := range collected {
		if !PlanDispatchedBy(secret, ownerKey) {
			continue
		}

		// A status which cannot be read is malformed bookkeeping, not a sign the plan is done, so the
		// plan is canceled and waited on anyway: asking the agent to stop is safe whatever state it is
		// in.
		planStatus, err := planapi.Status(secret)
		if err == nil && !planStatus.State.IsActive() {
			if checkpoint := planapi.ParsePlanCheckpoint(secret); checkpoint != nil && checkpoint.TerminationIncomplete {
				terminationIncomplete = append(terminationIncomplete, planNodeName(secret))
			}
			continue
		}
		running = append(running, planNodeName(secret))

		written, _, err := store.CancelPlan(secret, ownerKey)
		if err != nil {
			return false, err
		}
		if written {
			logrus.Infof("[operations] %s/%s: canceled the plan it dispatched to %s/%s",
				op.GetNamespace(), op.GetName(), secret.Namespace, secret.Name)
		}
	}

	if len(running) > 0 && stopClock().Before(status.LastUpdated.Add(DispatchedPlanStopTimeout)) {
		opv1alpha1.FinalizedCondition.False(status)
		opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.WaitingForPlansToStopReason)
		opv1alpha1.FinalizedCondition.Message(status, "waiting for the agents to stop the plans it dispatched to "+nodesSummary(running))

		return false, nil
	}

	var notes []string
	if len(running) > 0 {
		logrus.Warnf("[operations] %s/%s: the agents did not report stopping the plans on %v within %s; releasing the beacon anyway",
			op.GetNamespace(), op.GetName(), running, DispatchedPlanStopTimeout)
		notes = append(notes, fmt.Sprintf("the agents did not report stopping the plans on %s within %s",
			nodesSummary(running), DispatchedPlanStopTimeout))
	}
	if len(terminationIncomplete) > 0 {
		logrus.Warnf("[operations] %s/%s: the agents on %v could not confirm that every process the canceled plans started had exited",
			op.GetNamespace(), op.GetName(), terminationIncomplete)
		notes = append(notes, "work started by the canceled plans may still be running on "+nodesSummary(terminationIncomplete)+
			", whose agents could not confirm that every process they terminated had exited")
	}
	noteOutcome(status, notes)

	opv1alpha1.FinalizedCondition.False(status)
	opv1alpha1.FinalizedCondition.Reason(status, opv1alpha1.FinalizingReason)
	opv1alpha1.FinalizedCondition.Message(status, "waiting for terminal handling to complete")

	return true, nil
}

// noteOutcome appends notes to the message of the condition asserting the operation's outcome,
// which is where the record of how it ended is kept. Each note is added once, however often it is
// noted.
func noteOutcome(status *opv1alpha1.OperationStatus, notes []string) {
	if len(notes) == 0 {
		return
	}

	outcome, _ := opv1alpha1.OutcomeConditionFor(status.Phase)
	message := outcome.GetMessage(status)
	for _, note := range notes {
		if strings.Contains(message, note) {
			continue
		}
		if message == "" {
			message = note
		} else {
			message = message + "; " + note
		}
	}
	outcome.Message(status, message)
}

// planNodeName names the node a machine-plan secret belongs to, for messages: its machine, if it is
// labeled with one, otherwise the secret itself.
func planNodeName(secret *corev1.Secret) string {
	if machineName := secret.Labels[planv1alpha1.MachineLifecycleNameLabel]; machineName != "" {
		return machineName
	}
	return secret.Name
}

// nodesSummary names the first of nodes in lexicographic order and counts the rest:
// "X", "X & 1 other node" or "X & N other nodes".
func nodesSummary(nodes []string) string {
	nodes = slices.Clone(nodes)
	sort.Strings(nodes)

	switch len(nodes) {
	case 0:
		return ""
	case 1:
		return nodes[0]
	case 2:
		return nodes[0] + " & 1 other node"
	default:
		return fmt.Sprintf("%s & %d other nodes", nodes[0], len(nodes)-1)
	}
}

// PlanFailureMessage returns message, the message an operation is failed with because of
// planStatus, noting why when the plan did not simply fail.
//
// planapi.PlanStatus.Failure also reports a plan that was canceled, or that the agent left in a
// plan-state this build does not recognize, since neither will ever complete. They call for different
// follow-up from a failure: a failed plan points at the node, while a canceled one points at whoever
// canceled it. That is never the operation itself, which only cancels its plans once it has reached a
// terminal phase, and by then it is no longer waiting on any of them.
func PlanFailureMessage(planStatus *planapi.PlanStatus, message string) string {
	if planStatus == nil {
		return message
	}
	switch state := planStatus.State; {
	case state == planapi.PlanStateCanceled:
		return message + ": the in-progress plan was canceled externally"
	case state == planapi.PlanStateFailed, state == planapi.PlanStateSucceeded, state.IsActive():
		return message
	default:
		return fmt.Sprintf("%s: the agent reported plan-state %q, which is not recognized", message, state)
	}
}

// The progress buckets PlansMessage sorts plans into, in the order it reports them.
var planMessageBuckets = []string{
	"failing plan",
	"waiting for plan to be picked up",
	"waiting for plan applied",
	"plan paused",
	"waiting for probes",
}

// planMessageBucket returns the index into planMessageBuckets of the bucket a plan belongs in, and
// false for a plan that is done with, successfully or not.
func planMessageBucket(status planapi.PlanStatus) (int, bool) {
	switch {
	case !status.Waiting():
		return 0, false
	case status.Retrying:
		return 0, true
	case status.State == planapi.PlanStatePending:
		return 1, true
	case status.State == planapi.PlanStateInProgress:
		return 2, true
	case status.State == planapi.PlanStatePaused:
		return 3, true
	case status.State == planapi.PlanStateSucceeded:
		return 4, true
	}
	return 0, false
}

// PlansMessage aggregates the statuses of the plans an operation is waiting on into a single,
// human-readable status string.
//
// Nodes are bucketed by what their plan is waiting on, in this order:
//  1. failing plan (failed before, and being or about to be retried)
//  2. waiting for plan to be picked up (pending)
//  3. waiting for plan applied (in progress)
//  4. plan paused (paused)
//  5. waiting for probes (succeeded, probes not yet passed)
//
// Plans that are done with (successful, or failed with no retry to come, or canceled) are ignored.
// Within each bucket, node names are sorted lexicographically to guarantee deterministic outputs.
//
// Output string patterns adapt dynamically based on the node count per bucket:
//   - 1 node: "bucket_text for X"
//   - 2 nodes: "bucket_text for X & 1 other node"
//   - 3+ nodes: "bucket_text for X & N other nodes"
//
// The bucket summaries are joined with a comma and space, in the order above. Returns an empty string
// if results is empty or if no plan is still being waited on.
//
// As the result of this function may vary wildly between reconciliations, its intended purpose is to be used solely by
// handlers that have a fixed enqueue period to prevent thrashing. For example, a simple plan application for 100 nodes
// can cause at worst 100 status updates if each machine-plan state transition retriggers the handler if the message is
// used in a condition.
func PlansMessage(results []planapi.PlanStatus) string {
	buckets := make([][]string, len(planMessageBuckets))

	for _, res := range results {
		if res.Secret == nil {
			continue
		}
		bucket, ok := planMessageBucket(res)
		if !ok {
			continue
		}

		buckets[bucket] = append(buckets[bucket], planNodeName(res.Secret))
	}

	var messages []string
	for i, nodes := range buckets {
		if len(nodes) == 0 {
			continue
		}
		messages = append(messages, fmt.Sprintf("%s for %s", planMessageBuckets[i], nodesSummary(nodes)))
	}

	return strings.Join(messages, ", ")
}
