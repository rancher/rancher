package operations

import (
	"fmt"
	"strings"

	planapi "github.com/rancher/rancher/pkg/plan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationEnv returns environment variables which tie plan content to a specific operation and step.
//
// The system-agent only re-executes a plan when its serialized content changes, and AssignPlan only
// writes a plan when its bytes differ from the one already on the machine-plan secret. Without
// something operation-specific in the plan, a second operation which computes a byte-identical plan
// is reported as already applied on its first reconcile: its instructions never run again, and any
// saved output still belongs to the previous operation. Including these variables makes each
// operation's plans distinct, so every operation gets its own execution and its own output.
//
// controllerName is the controller's owner key (e.g. "etcd-snapshot-restore"), normalized to
// SCREAMING_SNAKE_CASE for the variable prefix. The variables themselves are inert on the node.
func OperationEnv[S ~string](controllerName string, op metav1.Object, step S) []string {
	prefix := strings.ToUpper(strings.ReplaceAll(controllerName, "-", "_"))
	return []string{
		fmt.Sprintf("%s_OPERATION_UID=%s", prefix, op.GetUID()),
		fmt.Sprintf("%s_STEP=%s", prefix, step),
	}
}

// WithOperationEnv appends env to every one-time and periodic instruction in p, and returns p so it
// can wrap a plan where it is assigned:
//
//	h.store.AssignPlan(secret, ops.WithOperationEnv(nodePlan, env), 1, 1)
//
// Applying the operation environment at the point of assignment rather than while each instruction is
// built keeps the guarantee — every assigned plan is scoped to its operation — in one place, and
// leaves plan builders free to be tested on their own terms. Returns nil for a nil plan so callers
// which build a plan conditionally can pass it through unchecked.
func WithOperationEnv(p *planapi.Plan, env []string) *planapi.Plan {
	if p == nil || len(env) == 0 {
		return p
	}
	for i := range p.OneTimeInstructions {
		p.OneTimeInstructions[i].Env = append(p.OneTimeInstructions[i].Env, env...)
	}
	for i := range p.PeriodicInstructions {
		p.PeriodicInstructions[i].Env = append(p.PeriodicInstructions[i].Env, env...)
	}
	return p
}
