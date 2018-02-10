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
	Active         *int64         `json:"active,omitempty"`
	CompletionTime string         `json:"completionTime,omitempty"`
	Conditions     []JobCondition `json:"conditions,omitempty"`
	Failed         *int64         `json:"failed,omitempty"`
	StartTime      string         `json:"startTime,omitempty"`
	Succeeded      *int64         `json:"succeeded,omitempty"`
}
