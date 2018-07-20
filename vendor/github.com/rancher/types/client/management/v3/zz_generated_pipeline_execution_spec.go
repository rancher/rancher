package client

const (
	PipelineExecutionSpecType               = "pipelineExecutionSpec"
	PipelineExecutionSpecFieldPipeline      = "pipeline"
	PipelineExecutionSpecFieldPipelineID    = "pipelineId"
	PipelineExecutionSpecFieldRun           = "run"
	PipelineExecutionSpecFieldTriggerUserID = "triggerUserId"
	PipelineExecutionSpecFieldTriggeredBy   = "triggeredBy"
)

type PipelineExecutionSpec struct {
	Pipeline      *Pipeline `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`
	PipelineID    string    `json:"pipelineId,omitempty" yaml:"pipelineId,omitempty"`
	Run           int64     `json:"run,omitempty" yaml:"run,omitempty"`
	TriggerUserID string    `json:"triggerUserId,omitempty" yaml:"triggerUserId,omitempty"`
	TriggeredBy   string    `json:"triggeredBy,omitempty" yaml:"triggeredBy,omitempty"`
}
