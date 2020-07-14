package client

const (
	ProjectAlertSpecType                       = "projectAlertSpec"
	ProjectAlertSpecFieldDescription           = "description"
	ProjectAlertSpecFieldDisplayName           = "displayName"
	ProjectAlertSpecFieldInitialWaitSeconds    = "initialWaitSeconds"
	ProjectAlertSpecFieldProjectID             = "projectId"
	ProjectAlertSpecFieldRecipients            = "recipients"
	ProjectAlertSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertSpecFieldSeverity              = "severity"
	ProjectAlertSpecFieldTargetPod             = "targetPod"
	ProjectAlertSpecFieldTargetWorkload        = "targetWorkload"
)

type ProjectAlertSpec struct {
	Description           string          `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName           string          `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	InitialWaitSeconds    int64           `json:"initialWaitSeconds,omitempty" yaml:"initialWaitSeconds,omitempty"`
	ProjectID             string          `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Recipients            []Recipient     `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	RepeatIntervalSeconds int64           `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string          `json:"severity,omitempty" yaml:"severity,omitempty"`
	TargetPod             *TargetPod      `json:"targetPod,omitempty" yaml:"targetPod,omitempty"`
	TargetWorkload        *TargetWorkload `json:"targetWorkload,omitempty" yaml:"targetWorkload,omitempty"`
}
