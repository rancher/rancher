package client

const (
	PipelineStatusType                 = "pipelineStatus"
	PipelineStatusFieldLastExecutionID = "lastExecutionId"
	PipelineStatusFieldLastRunState    = "lastRunState"
	PipelineStatusFieldLastStarted     = "lastStarted"
	PipelineStatusFieldNextRun         = "nextRun"
	PipelineStatusFieldNextStart       = "nextStart"
	PipelineStatusFieldPipelineState   = "pipelineState"
	PipelineStatusFieldToken           = "token"
	PipelineStatusFieldWebHookID       = "webhookId"
)

type PipelineStatus struct {
	LastExecutionID string `json:"lastExecutionId,omitempty" yaml:"lastExecutionId,omitempty"`
	LastRunState    string `json:"lastRunState,omitempty" yaml:"lastRunState,omitempty"`
	LastStarted     string `json:"lastStarted,omitempty" yaml:"lastStarted,omitempty"`
	NextRun         *int64 `json:"nextRun,omitempty" yaml:"nextRun,omitempty"`
	NextStart       string `json:"nextStart,omitempty" yaml:"nextStart,omitempty"`
	PipelineState   string `json:"pipelineState,omitempty" yaml:"pipelineState,omitempty"`
	Token           string `json:"token,omitempty" yaml:"token,omitempty"`
	WebHookID       string `json:"webhookId,omitempty" yaml:"webhookId,omitempty"`
}
