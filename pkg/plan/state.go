package plan

// PlanState represents the lifecycle state of a plan tracked in the plan Secret's data.
// The orchestrator writes Pending when delivering new plan content.
// The agent drives all subsequent transitions.
type PlanState string

const (
	// PlanStateKey is the Secret data key used to store the plan state.
	PlanStateKey = "plan-state"

	// PlanRevisionKey is the Secret data key for the plan revision counter.
	// The agent increments this each time it loads a new plan version for execution
	// (i.e. on the pending → in-progress transition). Orchestrators can read this
	// field to correlate which plan version was actually applied.
	PlanRevisionKey = "plan-revision"

	// PlanStatePending means the orchestrator has written new plan content and is
	// waiting for the agent to pick it up.
	PlanStatePending PlanState = "pending"

	// PlanStateInProgress means the agent has picked up the plan and is currently
	// executing it. If the agent crashes while in this state, it will re-execute
	// the plan from the beginning on the next startup.
	PlanStateInProgress PlanState = "in-progress"

	// PlanStateSucceeded means the agent has successfully completed all instructions
	// and probes in the plan.
	PlanStateSucceeded PlanState = "succeeded"

	// PlanStateFailed means the agent has exhausted its retry budget and the plan
	// has been marked as failed.
	PlanStateFailed PlanState = "failed"

	// PlanStateCancelled means the plan was cancelled via the
	// plan.cattle.io/cancelled: "true" annotation. This transition is handled by
	// the cancellation ticket and is listed here for completeness.
	PlanStateCancelled PlanState = "cancelled"
)

// IsTerminal returns true when the state is a terminal state (succeeded, failed, or cancelled).
// A terminal plan requires the orchestrator to write new plan content before the agent will
// act on it again.
func (s PlanState) IsTerminal() bool {
	return s == PlanStateSucceeded || s == PlanStateFailed || s == PlanStateCancelled
}
