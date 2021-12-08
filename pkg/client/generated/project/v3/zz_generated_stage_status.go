package client

const (
	StageStatusType         = "stageStatus"
	StageStatusFieldEnded   = "ended"
	StageStatusFieldStarted = "started"
	StageStatusFieldState   = "state"
	StageStatusFieldSteps   = "steps"
)

type StageStatus struct {
	Ended   string       `json:"ended,omitempty" yaml:"ended,omitempty"`
	Started string       `json:"started,omitempty" yaml:"started,omitempty"`
	State   string       `json:"state,omitempty" yaml:"state,omitempty"`
	Steps   []StepStatus `json:"steps,omitempty" yaml:"steps,omitempty"`
}
