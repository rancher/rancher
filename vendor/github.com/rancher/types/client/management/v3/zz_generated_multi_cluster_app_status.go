package client

const (
	MultiClusterAppStatusType            = "multiClusterAppStatus"
	MultiClusterAppStatusFieldConditions = "conditions"
	MultiClusterAppStatusFieldRevisionID = "revisionId"
)

type MultiClusterAppStatus struct {
	Conditions []AppCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	RevisionID string         `json:"revisionId,omitempty" yaml:"revisionId,omitempty"`
}
