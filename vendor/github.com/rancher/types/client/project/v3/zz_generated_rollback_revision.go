package client

const (
	RollbackRevisionType            = "rollbackRevision"
	RollbackRevisionFieldRevisionId = "revisionId"
)

type RollbackRevision struct {
	RevisionId string `json:"revisionId,omitempty" yaml:"revisionId,omitempty"`
}
