package v1alpha1

import (
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

	// TTL is the time-to-live for the operation in seconds.
	// This TTL is only enforced when the operation is not paused and has reached a terminal state.
	// Setting a value < 0 represents +infinity, i.e. an operation which does not expire.
	// The default value is `0`.
	// A value == 0 expires immediately.
	// +optional
	TTL int64 `json:"ttl,omitempty"`
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

	// OperationPhaseCanceled indicates the operation was canceled by the user or system.
	OperationPhaseCanceled OperationPhase = "Canceled"
)

// OperationStatus defines the observed state of an operation.
type OperationStatus struct {
	// Conditions represent the latest available observations of an operation's current state.
	// Known condition types are Pending, InProgress, Succeeded, Failed, Canceled, and Paused .
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

	// Phase represents the current phase of the Operation.
	// A Pending operation is one that is currently waiting to acquire the beacon, active it, and begin execution.
	// An InProgress operation is one that is currently executing.
	// A Succeeded operation is one that completed successfully.
	// A Failed operation is one that failed to complete successfully.
	// A Canceled operation is one that was canceled by the user or system.
	// +kubebuilder:validation:Enum=Pending;InProgress;Succeeded;Failed;Canceled
	// +optional
	Phase OperationPhase `json:"phase,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
