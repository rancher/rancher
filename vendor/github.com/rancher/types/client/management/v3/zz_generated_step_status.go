package client

const (
	StepStatusType         = "stepStatus"
	StepStatusFieldEnded   = "ended"
	StepStatusFieldStarted = "started"
	StepStatusFieldState   = "state"
)

type StepStatus struct {
	Ended   string `json:"ended,omitempty"`
	Started string `json:"started,omitempty"`
	State   string `json:"state,omitempty"`
}
