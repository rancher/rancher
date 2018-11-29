package client

const (
	MonitoringInputType         = "monitoringInput"
	MonitoringInputFieldAnswers = "answers"
)

type MonitoringInput struct {
	Answers map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
}
