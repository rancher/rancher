package client

const (
	PipelineStatusType                 = "pipelineStatus"
	PipelineStatusFieldLastExecutionID = "lastExecutionId"
	PipelineStatusFieldLastRunState    = "lastRunState"
	PipelineStatusFieldLastStarted     = "lastStarted"
	PipelineStatusFieldNextRun         = "nextRun"
	PipelineStatusFieldNextStart       = "nextStart"
	PipelineStatusFieldState           = "state"
	PipelineStatusFieldToken           = "token"
	PipelineStatusFieldWebHookID       = "webhookId"
)

type PipelineStatus struct {
	LastExecutionID string `json:"lastExecutionId,omitempty"`
	LastRunState    string `json:"lastRunState,omitempty"`
	LastStarted     string `json:"lastStarted,omitempty"`
	NextRun         *int64 `json:"nextRun,omitempty"`
	NextStart       string `json:"nextStart,omitempty"`
	State           string `json:"state,omitempty"`
	Token           string `json:"token,omitempty"`
	WebHookID       string `json:"webhookId,omitempty"`
}
