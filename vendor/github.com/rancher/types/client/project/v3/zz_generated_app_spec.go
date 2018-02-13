package client

const (
	AppSpecType                  = "appSpec"
	AppSpecFieldAnswers          = "answers"
	AppSpecFieldDescription      = "description"
	AppSpecFieldExternalID       = "externalId"
	AppSpecFieldGroups           = "groups"
	AppSpecFieldInstallNamespace = "installNamespace"
	AppSpecFieldProjectId        = "projectId"
	AppSpecFieldPrune            = "prune"
	AppSpecFieldTag              = "tag"
	AppSpecFieldTemplates        = "templates"
	AppSpecFieldUser             = "user"
)

type AppSpec struct {
	Answers          map[string]string `json:"answers,omitempty"`
	Description      string            `json:"description,omitempty"`
	ExternalID       string            `json:"externalId,omitempty"`
	Groups           []string          `json:"groups,omitempty"`
	InstallNamespace string            `json:"installNamespace,omitempty"`
	ProjectId        string            `json:"projectId,omitempty"`
	Prune            bool              `json:"prune,omitempty"`
	Tag              map[string]string `json:"tag,omitempty"`
	Templates        map[string]string `json:"templates,omitempty"`
	User             string            `json:"user,omitempty"`
}
