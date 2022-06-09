package client

const (
	MonitoringInputType                  = "monitoringInput"
	MonitoringInputFieldAnswers          = "answers"
	MonitoringInputFieldAnswersSetString = "answersSetString"
	MonitoringInputFieldVersion          = "version"
)

type MonitoringInput struct {
	Answers          map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersSetString map[string]string `json:"answersSetString,omitempty" yaml:"answersSetString,omitempty"`
	Version          string            `json:"version,omitempty" yaml:"version,omitempty"`
}
