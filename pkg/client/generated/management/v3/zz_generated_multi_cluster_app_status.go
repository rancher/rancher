package client

const (
	MultiClusterAppStatusType             = "multiClusterAppStatus"
	MultiClusterAppStatusFieldConditions  = "conditions"
	MultiClusterAppStatusFieldHelmVersion = "helmVersion"
	MultiClusterAppStatusFieldRevisionID  = "revisionId"
)

type MultiClusterAppStatus struct {
	Conditions  []AppCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	HelmVersion string         `json:"helmVersion,omitempty" yaml:"helmVersion,omitempty"`
	RevisionID  string         `json:"revisionId,omitempty" yaml:"revisionId,omitempty"`
}
