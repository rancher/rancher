package client

const (
	MonitoringOutputType                  = "monitoringOutput"
	MonitoringOutputFieldAnswers          = "answers"
	MonitoringOutputFieldAnswersSetString = "answersSetString"
	MonitoringOutputFieldVersion          = "version"
)

type MonitoringOutput struct {
	Answers          map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersSetString map[string]string `json:"answersSetString,omitempty" yaml:"answersSetString,omitempty"`
	Version          string            `json:"version,omitempty" yaml:"version,omitempty"`
}
