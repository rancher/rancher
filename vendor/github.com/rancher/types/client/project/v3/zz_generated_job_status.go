package client

const (
	JobStatusType                = "jobStatus"
	JobStatusFieldActive         = "active"
	JobStatusFieldCompletionTime = "completionTime"
	JobStatusFieldConditions     = "conditions"
	JobStatusFieldFailed         = "failed"
	JobStatusFieldStartTime      = "startTime"
	JobStatusFieldSucceeded      = "succeeded"
)

type JobStatus struct {
	Active         *int64         `json:"active,omitempty" yaml:"active,omitempty"`
	CompletionTime string         `json:"completionTime,omitempty" yaml:"completionTime,omitempty"`
	Conditions     []JobCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Failed         *int64         `json:"failed,omitempty" yaml:"failed,omitempty"`
	StartTime      string         `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	Succeeded      *int64         `json:"succeeded,omitempty" yaml:"succeeded,omitempty"`
}
