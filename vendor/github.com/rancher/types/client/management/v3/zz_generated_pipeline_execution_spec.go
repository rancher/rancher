package client

const (
	PipelineExecutionSpecType               = "pipelineExecutionSpec"
	PipelineExecutionSpecFieldPipeline      = "pipeline"
	PipelineExecutionSpecFieldPipelineId    = "pipelineId"
	PipelineExecutionSpecFieldRun           = "run"
	PipelineExecutionSpecFieldTriggerUserId = "triggerUserId"
	PipelineExecutionSpecFieldTriggeredBy   = "triggeredBy"
)

type PipelineExecutionSpec struct {
	Pipeline      *Pipeline `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`
	PipelineId    string    `json:"pipelineId,omitempty" yaml:"pipelineId,omitempty"`
	Run           *int64    `json:"run,omitempty" yaml:"run,omitempty"`
	TriggerUserId string    `json:"triggerUserId,omitempty" yaml:"triggerUserId,omitempty"`
	TriggeredBy   string    `json:"triggeredBy,omitempty" yaml:"triggeredBy,omitempty"`
}
