package client

const (
	PipelineExecutionStatusType         = "pipelineExecutionStatus"
	PipelineExecutionStatusFieldCommit  = "commit"
	PipelineExecutionStatusFieldEnded   = "ended"
	PipelineExecutionStatusFieldStages  = "stages"
	PipelineExecutionStatusFieldStarted = "started"
	PipelineExecutionStatusFieldState   = "state"
)

type PipelineExecutionStatus struct {
	Commit  string        `json:"commit,omitempty"`
	Ended   string        `json:"ended,omitempty"`
	Stages  []StageStatus `json:"stages,omitempty"`
	Started string        `json:"started,omitempty"`
	State   string        `json:"state,omitempty"`
}
