package client

const (
	AppSpecType                   = "appSpec"
	AppSpecFieldAnswers           = "answers"
	AppSpecFieldAnswersSetString  = "answersSetString"
	AppSpecFieldAppRevisionID     = "appRevisionId"
	AppSpecFieldDescription       = "description"
	AppSpecFieldExternalID        = "externalId"
	AppSpecFieldFiles             = "files"
	AppSpecFieldMaxRevisionCount  = "maxRevisionCount"
	AppSpecFieldMultiClusterAppID = "multiClusterAppId"
	AppSpecFieldProjectID         = "projectId"
	AppSpecFieldPrune             = "prune"
	AppSpecFieldTargetNamespace   = "targetNamespace"
	AppSpecFieldTimeout           = "timeout"
	AppSpecFieldValuesYaml        = "valuesYaml"
	AppSpecFieldWait              = "wait"
)

type AppSpec struct {
	Answers           map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersSetString  map[string]string `json:"answersSetString,omitempty" yaml:"answersSetString,omitempty"`
	AppRevisionID     string            `json:"appRevisionId,omitempty" yaml:"appRevisionId,omitempty"`
	Description       string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalID        string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files             map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	MaxRevisionCount  int64             `json:"maxRevisionCount,omitempty" yaml:"maxRevisionCount,omitempty"`
	MultiClusterAppID string            `json:"multiClusterAppId,omitempty" yaml:"multiClusterAppId,omitempty"`
	ProjectID         string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Prune             bool              `json:"prune,omitempty" yaml:"prune,omitempty"`
	TargetNamespace   string            `json:"targetNamespace,omitempty" yaml:"targetNamespace,omitempty"`
	Timeout           int64             `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ValuesYaml        string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
	Wait              bool              `json:"wait,omitempty" yaml:"wait,omitempty"`
}
