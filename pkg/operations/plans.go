package operations

import (
	"fmt"
	"sort"
	"strings"

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

// CancelDispatchedPlans asks the agents to abort every plan the operation holding the beacon under
// ownerKey dispatched to the given cluster and has not yet seen finish, and reports how many it had
// to cancel.
//
// This is what makes an operation's terminal outcome mean something on the cluster rather than only
// in its status. Marking the operation Canceled or Failed stops the controller dispatching anything
// further, but a plan already sitting in a machine-plan secret is the agent's to run, and it will
// keep running it — so an operation that released the beacon on the strength of its status alone
// would let the next operation start while the previous one's instructions were still executing.
// Canceling the plans first is what closes that window. See planapi.PlanCanceledAnnotation.
//
// Only plans the agent may still act on are canceled (see planapi.PlanState.IsActive). One that has
// finished has nothing left to stop: a succeeded plan's one-time instructions have all run, and
// canceling it would only stop its periodic instructions and record a cancellation of work which
// in fact completed.
//
// Only plans this operation wrote are touched, so a secret another operation has since taken over is
// left to it. And nothing is written at all unless the operation is still the beacon's current
// holder: a write on behalf of an operation that has lost the beacon would be reaching into the work
// of whoever holds it now, and the machine-plan webhook rejects one which slips through between this
// check and the write. "Current holder" is planapi.AuthorizedForBeacon: the beacon's owner, or the
// delegate it was last handed to. An operation part-way down the delegate chain has handed its
// authority on without giving the beacon up, so this returns an error for the caller to try again
// once it is handed back; one with no claim on the beacon at all has nothing it is entitled to
// cancel, and is passed over.
//
// There is nothing to cancel without a cluster to enumerate secrets under or a store to write
// through, so it does nothing rather than failing: an operation whose cluster has gone has no agent
// left to run anything it dispatched either.
func CancelDispatchedPlans(store *planapi.Store, secrets planapi.SecretClient, cluster *unstructured.Unstructured, namespace string, op metav1.Object, ownerKey string, beacon *planv1alpha1.Beacon) (int, error) {
	if store == nil || secrets == nil || cluster == nil || op == nil || ownerKey == "" {
		return 0, nil
	}

	if !planapi.HoldsBeacon(beacon, ownerKey) {
		logrus.Infof("[operations] %s/%s: no longer holds the beacon, leaving the plans it dispatched to whoever holds it now",
			op.GetNamespace(), op.GetName())

		return 0, nil
	}
	if !planapi.AuthorizedForBeacon(beacon, ownerKey) {
		return 0, fmt.Errorf("cannot cancel the plans %s/%s dispatched while it has handed the beacon on to a delegate",
			op.GetNamespace(), op.GetName())
	}

	collected, err := planapi.NewCollector(secrets, cluster, namespace).Collect()
	if planapi.IsTransient(err) {
		return 0, err
	} else if err != nil {
		// The machine-plan secrets cannot be enumerated at all and no retry will change that, so
		// this is reported and stepped over rather than returned. The operation still has to be able
		// to finish: refusing to would hold the beacon forever, freezing the cluster for every
		// operation behind it, with no way out but deleting this one — which arrives back here and
		// fails in exactly the same way.
		logrus.Errorf("[operations] %s/%s: cannot enumerate machine-plan secrets to cancel the plans it dispatched, any still in flight will run to completion: %v",
			op.GetNamespace(), op.GetName(), err)

		return 0, nil
	}

	canceled := 0

	for _, secret := range collected {
		if !PlanDispatchedBy(secret, ownerKey) {
			continue
		}

		// A status which cannot be read is malformed bookkeeping, not a sign the plan is done, so the
		// plan is canceled anyway: asking the agent to stop is safe whatever state it is in.
		if status, err := planapi.Status(secret); err == nil && !status.State.IsActive() {
			continue
		}

		written, _, err := store.CancelPlan(secret, ownerKey)
		if err != nil {
			return canceled, err
		}
		if written {
			logrus.Infof("[operations] %s/%s: canceled the plan it dispatched to %s/%s",
				op.GetNamespace(), op.GetName(), secret.Namespace, secret.Name)
			canceled++
		}
	}

	return canceled, nil
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

		name := res.Secret.Name
		if machineName := res.Secret.Labels[planv1alpha1.MachineLifecycleNameLabel]; machineName != "" {
			name = machineName
		}
		buckets[bucket] = append(buckets[bucket], name)
	}

	var messages []string
	for i, nodes := range buckets {
		if len(nodes) == 0 {
			continue
		}

		// Sort node names lexicographically within their own bucket
		sort.Strings(nodes)

		switch len(nodes) {
		case 1:
			messages = append(messages, fmt.Sprintf("%s for %s", planMessageBuckets[i], nodes[0]))
		case 2:
			messages = append(messages, fmt.Sprintf("%s for %s & 1 other node", planMessageBuckets[i], nodes[0]))
		default:
			messages = append(messages, fmt.Sprintf("%s for %s & %d other nodes", planMessageBuckets[i], nodes[0], len(nodes)-1))
		}
	}

	return strings.Join(messages, ", ")
}
