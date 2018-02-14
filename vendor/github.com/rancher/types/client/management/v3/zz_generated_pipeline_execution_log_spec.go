package client

const (
	PipelineExecutionLogSpecType                     = "pipelineExecutionLogSpec"
	PipelineExecutionLogSpecFieldLine                = "line"
	PipelineExecutionLogSpecFieldMessage             = "message"
	PipelineExecutionLogSpecFieldPipelineExecutionId = "pipelineExecutionId"
	PipelineExecutionLogSpecFieldProjectId           = "projectId"
	PipelineExecutionLogSpecFieldStage               = "stage"
	PipelineExecutionLogSpecFieldStep                = "step"
)

type PipelineExecutionLogSpec struct {
	Line                *int64 `json:"line,omitempty"`
	Message             string `json:"message,omitempty"`
	PipelineExecutionId string `json:"pipelineExecutionId,omitempty"`
	ProjectId           string `json:"projectId,omitempty"`
	Stage               *int64 `json:"stage,omitempty"`
	Step                *int64 `json:"step,omitempty"`
}
