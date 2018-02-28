package client

const (
	StepStatusType         = "stepStatus"
	StepStatusFieldEnded   = "ended"
	StepStatusFieldStarted = "started"
	StepStatusFieldState   = "state"
)

type StepStatus struct {
	Ended   string `json:"ended,omitempty" yaml:"ended,omitempty"`
	Started string `json:"started,omitempty" yaml:"started,omitempty"`
	State   string `json:"state,omitempty" yaml:"state,omitempty"`
}
