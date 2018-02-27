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
	AnswerValues     string            `json:"answerValues,omitempty"`
	Answers          map[string]string `json:"answers,omitempty"`
	Description      string            `json:"description,omitempty"`
	ExternalID       string            `json:"externalId,omitempty"`
	InstallNamespace string            `json:"installNamespace,omitempty"`
	ProjectId        string            `json:"projectId,omitempty"`
	Templates        map[string]string `json:"templates,omitempty"`
}
