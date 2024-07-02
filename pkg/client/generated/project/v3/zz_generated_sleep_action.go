package client

const (
	SleepActionType         = "sleepAction"
	SleepActionFieldSeconds = "seconds"
)

type SleepAction struct {
	Seconds int64 `json:"seconds,omitempty" yaml:"seconds,omitempty"`
}
