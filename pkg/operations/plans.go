package operations

import (
	"encoding/json"
	"strings"

	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// operationUIDEnvSuffix is the tail of the environment entry OperationEnv stamps onto every
// instruction of every plan an operation dispatches. The head is the controller's own name, so
// matching on the tail identifies the operation a plan belongs to without knowing which controller
// dispatched it.
const operationUIDEnvSuffix = "_OPERATION_UID="

// PlanDispatchedBy reports whether the plan currently assigned to secret was dispatched by op.
//
// The operation environment is the record of that: WithOperationEnv stamps the operation's UID onto
// every instruction at assignment time, so a plan carrying it is one this operation put there. A
// secret which has since been reassigned by another operation carries that operation's UID instead
// and is not claimed here, which is what makes acting on this safe for an operation that no longer
// holds the beacon.
//
// A secret with no plan, or one whose plan cannot be decoded, belongs to nobody: there is nothing
// this operation could usefully do to it either way.
func PlanDispatchedBy(secret *corev1.Secret, op metav1.Object) bool {
	if secret == nil || op == nil {
		return false
	}

	data := secret.Data[planapi.PlanDataKey]
	if len(data) == 0 {
		return false
	}

	var assigned planapi.Plan
	if err := json.Unmarshal(data, &assigned); err != nil {
		return false
	}

	marker := operationUIDEnvSuffix + string(op.GetUID())

	for _, instruction := range assigned.OneTimeInstructions {
		for _, env := range instruction.Env {
			if strings.HasSuffix(env, marker) {
				return true
			}
		}
	}

	for _, instruction := range assigned.PeriodicInstructions {
		for _, env := range instruction.Env {
			if strings.HasSuffix(env, marker) {
				return true
			}
		}
	}

	return false
}

// CancelDispatchedPlans asks the agents to abort every plan op dispatched to the given cluster, and
// reports how many it had to cancel.
//
// This is what makes a canceled operation's cancellation mean something on the cluster rather than
// only in its status. Marking the operation Canceled stops the controller dispatching anything
// further, but a plan already sitting in a machine-plan secret is the agent's to run, and it will
// keep running it — so an operation that released the beacon on the strength of its status alone
// would let the next operation start while the previous one's instructions were still executing.
// Canceling the plans first is what closes that window. See planapi.PlanCanceledAnnotation.
//
// Only plans carrying this operation's UID are touched, so a secret another operation has since
// taken over is left to it. That is also what makes this safe to run for an operation which no
// longer holds the beacon: it can only ever reach its own plans.
//
// There is nothing to cancel without a cluster to enumerate secrets under or a store to write
// through, so it does nothing rather than failing: an operation whose cluster has gone has no agent
// left to run anything it dispatched either.
func CancelDispatchedPlans(store *planapi.Store, secrets planapi.SecretClient, cluster *unstructured.Unstructured, namespace string, op metav1.Object) (int, error) {
	if store == nil || secrets == nil || cluster == nil || op == nil {
		return 0, nil
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
		if !PlanDispatchedBy(secret, op) {
			continue
		}

		written, _, err := store.CancelPlan(secret)
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
