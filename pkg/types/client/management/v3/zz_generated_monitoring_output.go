package client

const (
	MonitoringOutputType         = "monitoringOutput"
	MonitoringOutputFieldAnswers = "answers"
	MonitoringOutputFieldVersion = "version"
)

type MonitoringOutput struct {
	Answers map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Version string            `json:"version,omitempty" yaml:"version,omitempty"`
}
