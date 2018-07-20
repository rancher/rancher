package client

const (
	AppRevisionStatusType            = "appRevisionStatus"
	AppRevisionStatusFieldAnswers    = "answers"
	AppRevisionStatusFieldDigest     = "digest"
	AppRevisionStatusFieldExternalID = "externalId"
	AppRevisionStatusFieldProjectID  = "projectId"
)

type AppRevisionStatus struct {
	Answers    map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Digest     string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	ProjectID  string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
