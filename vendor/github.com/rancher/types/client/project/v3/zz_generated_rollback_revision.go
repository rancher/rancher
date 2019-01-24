package client

const (
	RollbackRevisionType              = "rollbackRevision"
	RollbackRevisionFieldForceUpgrade = "forceUpgrade"
	RollbackRevisionFieldRevisionID   = "revisionId"
)

type RollbackRevision struct {
	ForceUpgrade bool   `json:"forceUpgrade,omitempty" yaml:"forceUpgrade,omitempty"`
	RevisionID   string `json:"revisionId,omitempty" yaml:"revisionId,omitempty"`
}
