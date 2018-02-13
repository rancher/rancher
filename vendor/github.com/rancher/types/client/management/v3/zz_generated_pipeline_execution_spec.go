package client

const (
	PipelineExecutionSpecType               = "pipelineExecutionSpec"
	PipelineExecutionSpecFieldPipeline      = "pipeline"
	PipelineExecutionSpecFieldPipelineId    = "pipelineId"
	PipelineExecutionSpecFieldProjectId     = "projectId"
	PipelineExecutionSpecFieldRun           = "run"
	PipelineExecutionSpecFieldTriggerUserId = "triggerUserId"
	PipelineExecutionSpecFieldTriggeredBy   = "triggeredBy"
)

type PipelineExecutionSpec struct {
	Pipeline      *Pipeline `json:"pipeline,omitempty"`
	PipelineId    string    `json:"pipelineId,omitempty"`
	ProjectId     string    `json:"projectId,omitempty"`
	Run           *int64    `json:"run,omitempty"`
	TriggerUserId string    `json:"triggerUserId,omitempty"`
	TriggeredBy   string    `json:"triggeredBy,omitempty"`
}
