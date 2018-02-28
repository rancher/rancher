package client

const (
	AppSpecType                  = "appSpec"
	AppSpecFieldAnswerValues     = "answerValues"
	AppSpecFieldAnswers          = "answers"
	AppSpecFieldDescription      = "description"
	AppSpecFieldExternalID       = "externalId"
	AppSpecFieldInstallNamespace = "installNamespace"
	AppSpecFieldProjectId        = "projectId"
	AppSpecFieldTemplates        = "templates"
)

type AppSpec struct {
	AnswerValues     string            `json:"answerValues,omitempty" yaml:"answerValues,omitempty"`
	Answers          map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID       string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	InstallNamespace string            `json:"installNamespace,omitempty" yaml:"installNamespace,omitempty"`
	ProjectId        string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Templates        map[string]string `json:"templates,omitempty" yaml:"templates,omitempty"`
}
