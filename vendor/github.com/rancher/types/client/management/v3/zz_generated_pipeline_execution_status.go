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
	Commit         string        `json:"commit,omitempty"`
	Ended          string        `json:"ended,omitempty"`
	ExecutionState string        `json:"executionState,omitempty"`
	Stages         []StageStatus `json:"stages,omitempty"`
	Started        string        `json:"started,omitempty"`
}
