package client

const (
	PipelineExecutionStatusType                = "pipelineExecutionStatus"
	PipelineExecutionStatusFieldCommit         = "commit"
	PipelineExecutionStatusFieldConditions     = "conditions"
	PipelineExecutionStatusFieldEnded          = "ended"
	PipelineExecutionStatusFieldEnvVars        = "envVars"
	PipelineExecutionStatusFieldExecutionState = "executionState"
	PipelineExecutionStatusFieldStages         = "stages"
	PipelineExecutionStatusFieldStarted        = "started"
)

type PipelineExecutionStatus struct {
	Commit         string              `json:"commit,omitempty" yaml:"commit,omitempty"`
	Conditions     []PipelineCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Ended          string              `json:"ended,omitempty" yaml:"ended,omitempty"`
	EnvVars        map[string]string   `json:"envVars,omitempty" yaml:"envVars,omitempty"`
	ExecutionState string              `json:"executionState,omitempty" yaml:"executionState,omitempty"`
	Stages         []StageStatus       `json:"stages,omitempty" yaml:"stages,omitempty"`
	Started        string              `json:"started,omitempty" yaml:"started,omitempty"`
}
