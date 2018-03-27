package client

const (
	PipelineExecutionLogSpecType                     = "pipelineExecutionLogSpec"
	PipelineExecutionLogSpecFieldLine                = "line"
	PipelineExecutionLogSpecFieldMessage             = "message"
	PipelineExecutionLogSpecFieldPipelineExecutionId = "pipelineExecutionId"
	PipelineExecutionLogSpecFieldStage               = "stage"
	PipelineExecutionLogSpecFieldStep                = "step"
)

type PipelineExecutionLogSpec struct {
	Line                int64  `json:"line,omitempty" yaml:"line,omitempty"`
	Message             string `json:"message,omitempty" yaml:"message,omitempty"`
	PipelineExecutionId string `json:"pipelineExecutionId,omitempty" yaml:"pipelineExecutionId,omitempty"`
	Stage               int64  `json:"stage,omitempty" yaml:"stage,omitempty"`
	Step                int64  `json:"step,omitempty" yaml:"step,omitempty"`
}
