package v1alpha1

import (
	"fmt"

	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationSpec defines the shared configuration required for an operation.
// Operations must embed the OperationSpec struct within their respective spec definitions; direct consumption is also
// acceptable.
type OperationSpec struct {
	// ClusterRef is a reference to the Cluster this operation is associated with.
	// +required
	ClusterRef *corev1.ObjectReference `json:"clusterRef,omitempty"`

	// Paused indicates whether the operation is paused.
	// When paused, the operation will halt execution.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Cancel requests the operation to stop permanently. Unlike Paused, it is terminal and cannot be unset.
	// Recover by deleting and recreating the operation.
	// Paused takes precedence: a paused operation halts reconciliation entirely, so the cancellation
	// is only observed once the pause is lifted.
	// Canceling an operation which has already reached a terminal phase does nothing: there is no
	// work left to call off, so the phase it ended in stands and the Canceled condition reports that
	// the request was declined. Deleting such an operation does still stop it, which is the remedy
	// for one whose terminal phase hook is never answered.
	// Setting it on an operation already in a terminal phase is rejected outright, by a validation
	// rule which has to live on each operation type rather than here: a rule on this field only sees
	// the field, and deciding whether a cancellation can still take effect needs status.phase. The
	// rule cannot close the race where an operation reaches a terminal phase between the request
	// being admitted and the controller acting on it, which is why the controller declines it too.
	// +kubebuilder:default=false
	// +kubebuilder:validation:XValidation:rule="self || !oldSelf",message="cancel cannot be unset once true"
	// +optional
	Cancel bool `json:"cancel,omitempty"`

	// TTL is the time-to-live for the operation in seconds.
	// This TTL is only enforced when the operation is not paused and has reached a terminal state.
	// Setting a value < 0 represents +infinity, i.e. an operation which does not expire.
	// The default value is `0`.
	// A value == 0 expires immediately.
	// +optional
	TTL int64 `json:"ttl,omitempty"`
}

// ClusterRefKey renders a cluster reference for logs and status messages, omitting the namespace
// for cluster-scoped references.
func ClusterRefKey(ref *corev1.ObjectReference) string {
	key := fmt.Sprintf("apiVersion=%s, kind=%s", ref.APIVersion, ref.Kind)
	if ref.Namespace != "" {
		key += fmt.Sprintf(", namespace=%s", ref.Namespace)
	}
	return key + fmt.Sprintf(", name=%s", ref.Name)
}

// OperationPhase represents the current phase of the operation.
type OperationPhase string

const (
	// OperationPhasePending indicates the operation is waiting to be executed.
	OperationPhasePending OperationPhase = "Pending"

	// OperationPhaseInProgress indicates the operation is currently running.
	OperationPhaseInProgress OperationPhase = "InProgress"

	// OperationPhaseSucceeded indicates the operation completed successfully.
	OperationPhaseSucceeded OperationPhase = "Succeeded"

	// OperationPhaseFailed indicates the operation was unsuccessful.
	OperationPhaseFailed OperationPhase = "Failed"

	// OperationPhaseAborted indicates the operation called its own work off, having found a
	// condition it cannot proceed past (e.g. a failed preflight check). Nothing outside
	// the operation asked it to stop, and nothing it was asked to do was attempted and lost, which
	// is what separates Aborted from Canceled and Failed respectively.
	OperationPhaseAborted OperationPhase = "Aborted"

	// OperationPhaseCanceled indicates the operation was called off from outside: the user set
	// Cancel, another controller needed it to stop, or it was deleted before its terminal handling
	// completed. The work it had already dispatched is no longer tracked by anything, so it can be
	// reported neither as succeeded nor as failed.
	OperationPhaseCanceled OperationPhase = "Canceled"
)

// OperationStatus defines the observed state of an operation.
type OperationStatus struct {
	// Conditions represent the latest available observations of an operation's current state.
	// Known condition types are Pending, InProgress, Succeeded, Failed, Aborted, Canceled,
	// Finalized, and Paused.
	// Succeeded, Failed, Aborted and Canceled report how the operation ended, and the one matching the
	// terminal phase goes True as soon as that phase is reached. Finalized reports the separate
	// question of whether the controller has finished with the operation (terminal phase hook
	// satisfied, beacon released (see TerminatedAt)) and is True whenever any outcome condition is
	// and that work is done, so an observer that only needs to know the operation is over can wait
	// on it alone.
	// Operations may have additional conditions of their own.
	// Operations may also provide additional information in the form of messages.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// LastUpdated identifies when the phase of the Operation last transitioned.
	// LastUpdated will also be updated during step transitions, if applicable.
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty,omitzero"`

	// TerminatedAt identifies when the controller finished handling the terminal phase of the
	// Operation, meaning nothing is owed on it any more. Ordinarily the terminal-phase lifecycle
	// hook (if any) ran to completion and the beacon was released; it also covers an Operation with
	// no beacon left to release, and one being deleted, whose hooks are abandoned along with it.
	// It is set once and never cleared, and is only ever set on an Operation which has reached a
	// terminal phase.
	// An Operation which reached a terminal phase is not necessarily terminated: terminal handling
	// may still be delegated to another controller. An Operation deleted before it is terminated is
	// canceled, as the work it dispatched is no longer tracked by anything.
	// +optional
	TerminatedAt metav1.Time `json:"terminatedAt,omitempty,omitzero"`

	// Phase represents the current phase of the Operation.
	// A Pending operation is one that is currently waiting to acquire the beacon, active it, and begin execution.
	// An InProgress operation is one that is currently executing.
	// A Succeeded operation is one that completed successfully.
	// A Failed operation is one that failed to complete successfully.
	// An Aborted operation is one that called its own work off, having found it cannot proceed.
	// A Canceled operation is one that was called off from outside, by the user or the system.
	// +kubebuilder:validation:Enum=Pending;InProgress;Succeeded;Failed;Aborted;Canceled
	// +optional
	Phase OperationPhase `json:"phase,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SetPhase moves the operation to phase, recording when the transition happened. Setting the phase
// the operation is already in is a no-op, so LastUpdated keeps pointing at the moment the operation
// actually moved however many times it is reconciled afterward.
//
// The phase and its outcome condition are asserted together by the Mark* methods below; this is for
// the non-terminal transitions, which have no outcome to report.
func (s *OperationStatus) SetPhase(phase OperationPhase) {
	if s.Phase == phase {
		return
	}

	s.Phase = phase
	s.LastUpdated = metav1.Now()
}

// MarkSucceeded moves the operation to the Succeeded terminal phase, asserting the outcome: the work
// is over and the result will not change. Whether the controller is finished with the operation is
// reported separately, by the Finalized condition.
func (s *OperationStatus) MarkSucceeded() {
	s.markOutcome(OperationPhaseSucceeded, FinishedReason, "Operation completed successfully")
}

// MarkFailed moves the operation to the Failed terminal phase. Failed is what an operation reports
// when it gave up on its own work; the caller supplies the reason and message.
func (s *OperationStatus) MarkFailed(reason, message string) {
	s.markOutcome(OperationPhaseFailed, reason, message)
}

// MarkAborted moves the operation to the Aborted terminal phase, for work the operation called off
// itself after finding a condition it cannot proceed past (e.g. a failed preflight check).
// The corresponding downstream cluster must be in a healthy state, either equal or comparable to
// the state pre-operation.
func (s *OperationStatus) MarkAborted(reason, message string) {
	s.markOutcome(OperationPhaseAborted, reason, message)
}

// MarkCanceled moves the operation to the Canceled terminal phase, for work that was called off
// from outside the operation: spec.Cancel being set, or a deletion that raced the operation.
func (s *OperationStatus) MarkCanceled(reason, message string) {
	s.markOutcome(OperationPhaseCanceled, reason, message)
}

// markOutcome moves the operation to a terminal phase and records why on the condition that reports
// that phase. Doing both here is what keeps a phase and its outcome from disagreeing: the reason and
// message recorded at decision time are the operation's account of how it ended, and the controllers
// leave them alone from here on.
func (s *OperationStatus) markOutcome(phase OperationPhase, reason, message string) {
	s.SetPhase(phase)

	outcome, _ := OutcomeConditionFor(phase)
	outcome.True(s)
	outcome.Reason(s, reason)
	outcome.Message(s, message)
}

// SetTerminated records that terminal handling for the operation has completed. Operation
// controllers must only call this once nothing is owed on the operation (its terminal phase hook
// has been satisfied, or there is nothing left to hand one, or the operation is being deleted and
// its hooks go with it) and the beacon has been released. It is what makes the operation eligible
// for TTL garbage collection, and what distinguishes a deletion that races terminal handling
// (canceled) from one that follows it (left as-is).
//
// The timestamp is only written on the first call so it keeps pointing at the moment terminal
// handling actually completed, no matter how many times the operation is reconciled afterward.
func (s *OperationStatus) SetTerminated() {
	if !s.TerminatedAt.IsZero() {
		return
	}
	s.TerminatedAt = metav1.Now()
}
