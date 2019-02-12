package client

const (
	MonitoringOutputType         = "monitoringOutput"
	MonitoringOutputFieldAnswers = "answers"
)

type MonitoringOutput struct {
	Answers map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
}
