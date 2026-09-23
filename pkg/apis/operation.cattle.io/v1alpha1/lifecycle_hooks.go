package v1alpha1

// Phase hook label prefixes are the shared "<phase>.phase.hook.operation.cattle.io/" namespace used
// to gate operation progression at phase boundaries. They are common to every operation type
// (ETCDSnapshotSave, ETCDSnapshotRestore, EncryptionKeyRotation, …) because every operation goes
// through the same phase state machine (Pending → InProgress → Succeeded | Failed | Aborted, with
// Canceled reachable from any phase the controller has not finished handling yet).
// Step-level hooks are operation-specific and live alongside their controller.
//
// Semantics: when an Operation carries a label whose key starts with one of these prefixes, the
// owning controller pushes the label's value onto the cluster beacon's delegate chain on entry
// to the matching phase and then short-circuits further work in that phase. The operation
// resumes only after BOTH:
//
//   - the label is removed from the Operation (otherwise the next reconcile re-pushes the same
//     delegate onto the chain), and
//   - the delegate is popped from the beacon's delegate chain (otherwise the controller still
//     sees the delegate as the current beacon authority).
//
// The label-key suffix after the prefix is the hook's identifier (e.g.
// "<prefix>/my-cleanup-hook"); the controller does not interpret it. The label VALUE is the name
// of the delegate pushed onto the beacon chain — a cooperating controller subscribes to that
// delegate name to know when its turn arrives, and pops itself off the chain when finished.
//
// A hook on a terminal phase holds the beacon for as long as its delegate does not return it, and
// the beacon gates every operation on the cluster — so a delegate which never finishes blocks the
// next operation indefinitely, whatever its urgency. The operation reports which delegate it is
// waiting on through its Finalized condition. Setting spec.Cancel will not break the deadlock, as
// cancellation does not apply to an operation which has already reached a terminal phase; deleting
// the operation does, and is the supported remedy.
//
// Use these prefixes when adding cross-cutting hooks that should run at the same phase point
// across every operation type. For operation-type-specific gating (e.g. between snapshot Save and
// Restart, or between EKR Rotate and Restart) use the step-level prefixes exported by the
// respective controller package.
const (
	// PendingPhaseHookLabelPrefix gates the Pending phase, after the controller has acquired the
	// cluster beacon but before it waits for system-agents to register. A delegate hooked here
	// observes the cluster in its pre-operation state.
	PendingPhaseHookLabelPrefix = "pending.phase.hook.operation.cattle.io/"

	// InProgressPhaseHookLabelPrefix gates every InProgress reconcile, ahead of step dispatch.
	// Useful for delegates that need to gate ALL step work uniformly without subscribing to each
	// step prefix individually.
	InProgressPhaseHookLabelPrefix = "in-progress.phase.hook.operation.cattle.io/"

	// AbortedPhaseHookLabelPrefix gates the Aborted phase, before the controller releases the
	// beacon and runs any operation-type-specific cleanup (e.g. unpausing the CAPI cluster on
	// encryption-key-rotation). Lets a delegate inspect / react to the reason the operation called
	// its own work off.
	AbortedPhaseHookLabelPrefix = "aborted.phase.hook.operation.cattle.io/"

	// CanceledPhaseHookLabelPrefix gates the Canceled phase, before the controller releases the
	// beacon and runs any operation-type-specific cleanup (e.g. unpausing the CAPI cluster on
	// encryption-key-rotation). Lets a delegate inspect / react to the cancellation cause.
	//
	// Unlike the other phase hooks this one is honoured but not guaranteed: cancellation is driven
	// from outside the operation, often by whoever wants the beacon next, so the operation may no
	// longer hold the beacon by the time the hook would run. A controller which finds it holds no
	// claim on the beacon has no authority to delegate and skips the hook.
	CanceledPhaseHookLabelPrefix = "canceled.phase.hook.operation.cattle.io/"

	// FailedPhaseHookLabelPrefix gates the Failed phase, before the controller releases the
	// beacon and runs any operation-type-specific cleanup. Lets a delegate inspect failure state
	// (status conditions, plan-secret applied output, residual node-side scripts) before the next
	// operation is allowed to acquire the beacon.
	FailedPhaseHookLabelPrefix = "failed.phase.hook.operation.cattle.io/"

	// SucceededPhaseHookLabelPrefix gates the Succeeded phase, before the controller releases the
	// beacon. Lets a delegate chain follow-up work (e.g. snapshotbackpopulate after a restore, an
	// external verifier after a key rotation) before the cluster accepts new operations.
	SucceededPhaseHookLabelPrefix = "succeeded.phase.hook.operation.cattle.io/"
)

// LifecycleHookLabelMarker is the substring shared by every phase-hook and step-hook label key.
// Detecting it is enough to know "some delegate has posted a hook here" without enumerating every
// registered prefix — an operation-type-specific step prefix declared in a controller package still
// matches. See ops.HasActiveLifecycleHook.
const LifecycleHookLabelMarker = ".hook.operation.cattle.io/"
