package plan

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

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

	// PlanStateCanceled means the plan was canceled via the plan.cattle.io/canceled: "true" annotation.
	PlanStateCanceled PlanState = "canceled"

	// PlanStatePaused is a non-terminal state where execution is held at an instruction boundary.
	// Removing the paused annotation resumes execution using PlanCheckpoint.ResumeState.
	PlanStatePaused PlanState = "paused"
)

const (
	// PlanCheckpointKey is the Secret data key holding the resume checkpoint.
	PlanCheckpointKey = "plan-checkpoint"

	// PlanCanceledAnnotation is the Secret annotation used to cancel a plan.
	// Setting it to "true" requests that the agent abort the plan.
	// Removing the annotation does not resume the plan: cancellation is terminal and requires new content.
	// The only valid values are "true" and "false".
	PlanCanceledAnnotation = "plan.cattle.io/canceled"

	// PlanPausedAnnotation is the Secret annotation used to pause a plan.
	// Setting it to "true" requests that the agent stop executing the plan.
	// While set, the agent performs no plan execution regardless of plan state or resume checkpoint.
	// Clearing the annotation or setting it to "false" resumes the plan.
	// The only valid values are "true" and "false".
	PlanPausedAnnotation = "plan.cattle.io/paused"
)

// IsTerminal returns true when the state is a terminal state (succeeded, failed, or canceled).
// A terminal plan requires the orchestrator to write new plan content before the agent will
// act on it again.
func (s PlanState) IsTerminal() bool {
	return s == PlanStateSucceeded || s == PlanStateFailed || s == PlanStateCanceled
}

// PlanCheckpoint is the resume checkpoint stored under PlanCheckpointKey.
// A checkpoint is scoped to the plan checksum. A checkpoint from a different plan is ignored.
// This lets an agent resume a paused plan after a restart.
type PlanCheckpoint struct {
	Checksum    string    `json:"checksum,omitempty"`
	Completed   int       `json:"completedInstructions,omitempty"`
	Total       int       `json:"totalInstructions,omitempty"`
	ResumeState PlanState `json:"resumeState,omitempty"` // state restored when the pause lifts

	// Paused identifies the checkpoint as a suspension. Only suspended checkpoints can be used to resume.
	// Cancellation and resuming from a pause write Paused=false.
	Paused bool `json:"paused,omitempty"`

	// TerminationIncomplete reports that processes from an interrupted instruction may still run.
	// It is informational only; the agent does not act on it.
	// Unlike Paused it persists across checkpoint rewrites until the checkpoint is cleared.
	// It is a lower-bound signal: processes may still be running.
	TerminationIncomplete bool `json:"terminationIncomplete,omitempty"`
}

// ParsePlanCheckpoint decodes the resume checkpoint stored under PlanCheckpointKey.
// Return nil when the key is absent, empty, unparsable, or the checksum does not match the current plan.
func ParsePlanCheckpoint(secret *corev1.Secret) *PlanCheckpoint {
	if secret == nil {
		return nil
	}
	raw, ok := secret.Data[PlanCheckpointKey]
	if !ok || len(raw) == 0 {
		return nil
	}
	var p PlanCheckpoint
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	if p.Checksum == "" || p.Checksum != Checksum(secret.Data["plan"]) {
		return nil
	}
	return &p
}
