package client

const (
	AppRevisionStatusType            = "appRevisionStatus"
	AppRevisionStatusFieldAnswers    = "answers"
	AppRevisionStatusFieldDigest     = "digest"
	AppRevisionStatusFieldExternalID = "externalId"
	AppRevisionStatusFieldProjectId  = "projectId"
)

type AppRevisionStatus struct {
	Answers    map[string]string `json:"answers,omitempty" yaml:"answers,omitempty"`
	Digest     string            `json:"digest,omitempty" yaml:"digest,omitempty"`
	ExternalID string            `json:"externalId,omitempty" yaml:"externalId,omitempty"`
	ProjectId  string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
