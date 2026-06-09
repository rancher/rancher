package v1alpha1

import "github.com/rancher/wrangler/v3/pkg/condition"

var (
	// PendingCondition represents the condition state for a task or process that is awaiting execution or resolution.
	PendingCondition = condition.Cond("Pending")

	// InProgressCondition represents the condition state for a task or process that is currently in progress or being executed.
	InProgressCondition = condition.Cond("InProgress")

	// SucceededCondition represents the condition state for a task or process that completed successfully.
	SucceededCondition = condition.Cond("Succeeded")

	// FailedCondition represents the condition state for a task or process that has failed to complete successfully.
	FailedCondition = condition.Cond("Failed")

	// CanceledCondition represents the condition state for a task or process that has been canceled or terminated.
	CanceledCondition = condition.Cond("Canceled")

	// PausedCondition represents the condition state for a task or process that has been paused.
	PausedCondition = condition.Cond("Paused")
)

const (
	// ClusterNotFoundReason surfaces when an operation fails because the cluster is not found.
	ClusterNotFoundReason = "ClusterNotFound"

	// BeaconLostReason surfaces when an operation fails because the beacon is lost.
	BeaconLostReason = "BeaconLost"

	// UnknownStepReason surfaces when an operation fails because the step is unknown.
	UnknownStepReason = "UnknownStep"

	// UnknownPhaseReason surfaces when an operation fails because the phase is unknown.
	UnknownPhaseReason = "UnknownPhase"

	// WaitingForRegistrationReason surfaces when an operation is waiting for registration.
	WaitingForRegistrationReason = "WaitingForRegistration"

	// WaitingForBeaconReason surfaces when an operation is waiting to acquire the beacon.
	WaitingForBeaconReason = "WaitingForBeacon"

	// WaitingForPlanAppliedReason surfaces when an operation is waiting for a node plan to be applied.
	WaitingForPlanAppliedReason = "WaitingForPlanApplied"

	PlanFailedReason = "PlanFailed"

	// FinishedReason surfaces when an operation has reached a terminal state (success/failure).
	FinishedReason = "Finished"

	// NotFailedReason surfaces when an operation has not failed.
	NotFailedReason = "NotFailed"

	// NotSuccessfulReason surfaces when an operation has not completed successfully.
	NotSuccessfulReason = "notSuccessful"

	// InProgressReason surfaces when an operation is currently in progress.
	InProgressReason = "InProgress"

	// PausedReason surfaces when an operation is paused.
	PausedReason = "Paused"

	// NotPausedReason surfaces when an operation is not paused.
	NotPausedReason = "NotPaused"

	// WaitingForSuitableLeaderReason surfaces when no suitable control-plane leader can be
	// elected for encryption key rotation yet. The operation will retry automatically.
	WaitingForSuitableLeaderReason = "WaitingForSuitableLeader"

	// WaitingForEncryptionKeyRotationReason surfaces when the rotate-keys plan has been applied
	// but the runtime secrets-encrypt status has not yet confirmed reencrypt_finished.
	WaitingForEncryptionKeyRotationReason = "WaitingForEncryptionKeyRotation"
)
