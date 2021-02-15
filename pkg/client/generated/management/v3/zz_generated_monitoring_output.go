package client

const (
	MonitoringOutputType                    = "monitoringOutput"
	MonitoringOutputFieldAnswers            = "answers"
	MonitoringOutputFieldAnswersForceString = "answersForceString"
	MonitoringOutputFieldVersion            = "version"
)

type MonitoringOutput struct {
	Answers            map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersForceString map[string]bool   `json:"answersForceString,omitempty" yaml:"answersForceString,omitempty"`
	Version            string            `json:"version,omitempty" yaml:"version,omitempty"`
}
