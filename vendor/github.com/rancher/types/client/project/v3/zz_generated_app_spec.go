package client

const (
	AppSpecType                 = "appSpec"
	AppSpecFieldAnswers         = "answers"
	AppSpecFieldAppRevisionId   = "appRevisionId"
	AppSpecFieldDescription     = "description"
	AppSpecFieldExternalID      = "externalId"
	AppSpecFieldProjectId       = "projectId"
	AppSpecFieldPrune           = "prune"
	AppSpecFieldTargetNamespace = "targetNamespace"
)

type AppSpec struct {
	Answers         map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AppRevisionId   string            `json:"appRevisionId,omitempty" yaml:"appRevisionId,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID      string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	ProjectId       string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Prune           bool              `json:"prune,omitempty" yaml:"prune,omitempty"`
	TargetNamespace string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
}
