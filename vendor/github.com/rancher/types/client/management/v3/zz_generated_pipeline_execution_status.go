package client

const (
	PipelineExecutionStatusType                = "pipelineExecutionStatus"
	PipelineExecutionStatusFieldCommit         = "commit"
	PipelineExecutionStatusFieldEnded          = "ended"
	PipelineExecutionStatusFieldExecutionState = "executionState"
	PipelineExecutionStatusFieldStages         = "stages"
	PipelineExecutionStatusFieldStarted        = "started"
)

type PipelineExecutionStatus struct {
	Commit         string        `json:"commit,omitempty" yaml:"commit,omitempty"`
	Ended          string        `json:"ended,omitempty" yaml:"ended,omitempty"`
	ExecutionState string        `json:"executionState,omitempty" yaml:"executionState,omitempty"`
	Stages         []StageStatus `json:"stages,omitempty" yaml:"stages,omitempty"`
	Started        string        `json:"started,omitempty" yaml:"started,omitempty"`
}
