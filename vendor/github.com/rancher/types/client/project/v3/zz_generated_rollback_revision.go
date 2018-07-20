package client

const (
	RollbackRevisionType            = "rollbackRevision"
	RollbackRevisionFieldRevisionID = "revisionId"
)

type RollbackRevision struct {
	RevisionID string `json:"revisionId,omitempty" yaml:"revisionId,omitempty"`
}
