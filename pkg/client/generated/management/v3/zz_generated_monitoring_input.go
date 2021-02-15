package client

const (
	MonitoringInputType                    = "monitoringInput"
	MonitoringInputFieldAnswers            = "answers"
	MonitoringInputFieldAnswersForceString = "answersForceString"
	MonitoringInputFieldVersion            = "version"
)

type MonitoringInput struct {
	Answers            map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersForceString map[string]bool   `json:"answersForceString,omitempty" yaml:"answersForceString,omitempty"`
	Version            string            `json:"version,omitempty" yaml:"version,omitempty"`
}
