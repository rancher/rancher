package client

const (
	DurationType          = "duration"
	DurationFieldDuration = "duration"
)

type Duration struct {
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`
}
