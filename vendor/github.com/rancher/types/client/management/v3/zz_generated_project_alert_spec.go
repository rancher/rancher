package client

const (
	ProjectAlertSpecType                       = "projectAlertSpec"
	ProjectAlertSpecFieldDescription           = "description"
	ProjectAlertSpecFieldDisplayName           = "displayName"
	ProjectAlertSpecFieldInitialWaitSeconds    = "initialWaitSeconds"
	ProjectAlertSpecFieldProjectId             = "projectId"
	ProjectAlertSpecFieldRecipients            = "recipients"
	ProjectAlertSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertSpecFieldSeverity              = "severity"
	ProjectAlertSpecFieldTargetPod             = "targetPod"
	ProjectAlertSpecFieldTargetWorkload        = "targetWorkload"
)

type ProjectAlertSpec struct {
	Description           string          `json:"description,omitempty"`
	DisplayName           string          `json:"displayName,omitempty"`
	InitialWaitSeconds    *int64          `json:"initialWaitSeconds,omitempty"`
	ProjectId             string          `json:"projectId,omitempty"`
	Recipients            []Recipient     `json:"recipients,omitempty"`
	RepeatIntervalSeconds *int64          `json:"repeatIntervalSeconds,omitempty"`
	Severity              string          `json:"severity,omitempty"`
	TargetPod             *TargetPod      `json:"targetPod,omitempty"`
	TargetWorkload        *TargetWorkload `json:"targetWorkload,omitempty"`
}
