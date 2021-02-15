package client

const (
	AppRevisionStatusType                    = "appRevisionStatus"
	AppRevisionStatusFieldAnswers            = "answers"
	AppRevisionStatusFieldAnswersForceString = "answersForceString"
	AppRevisionStatusFieldDigest             = "digest"
	AppRevisionStatusFieldExternalID         = "externalId"
	AppRevisionStatusFieldFiles              = "files"
	AppRevisionStatusFieldProjectID          = "projectId"
	AppRevisionStatusFieldValuesYaml         = "valuesYaml"
)

type AppRevisionStatus struct {
	Answers            map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	AnswersForceString map[string]bool   `json:"answersForceString,omitempty" yaml:"answersForceString,omitempty"`
	Digest             string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID         string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	Files              map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
	ProjectID          string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	ValuesYaml         string            `json:"valuesYaml,omitempty" yaml:"valuesYaml,omitempty"`
}
