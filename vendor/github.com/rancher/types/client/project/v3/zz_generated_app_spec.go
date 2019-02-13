package client

const (
	AppSpecType                   = "appSpec"
	AppSpecFieldAnswers           = "answers"
	AppSpecFieldAppRevisionID     = "appRevisionId"
	AppSpecFieldDescription       = "description"
	AppSpecFieldExternalID        = "externalId"
	AppSpecFieldFiles             = "files"
	AppSpecFieldMultiClusterAppID = "multiClusterAppId"
	AppSpecFieldProjectID         = "projectId"
	AppSpecFieldPrune             = "prune"
	AppSpecFieldTargetNamespace   = "targetNamespace"
	AppSpecFieldValuesYaml        = "valuesYaml"
)

type AppSpec struct {
	Answers           map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AppRevisionID     string            `json:"appRevisionId,omitempty" yaml:"appRevisionId,omitempty"`
	Description       string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID        string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files             map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	MultiClusterAppID string            `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	ProjectID         string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Prune             bool              `json:"prune,omitempty" yaml:"prune,omitempty"`
	TargetNamespace   string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
	ValuesYaml        string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
}
