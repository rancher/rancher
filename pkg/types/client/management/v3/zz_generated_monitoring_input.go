package client

const (
	MonitoringInputType         = "monitoringInput"
	MonitoringInputFieldAnswers = "answers"
	MonitoringInputFieldVersion = "version"
)

type MonitoringInput struct {
	Answers map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Version string            `json:"version,omitempty" yaml:"version,omitempty"`
}
