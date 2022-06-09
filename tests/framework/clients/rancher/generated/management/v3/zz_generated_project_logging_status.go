package client

const (
	ProjectLoggingStatusType             = "projectLoggingStatus"
	ProjectLoggingStatusFieldAppliedSpec = "appliedSpec"
	ProjectLoggingStatusFieldConditions  = "conditions"
)

type ProjectLoggingStatus struct {
	AppliedSpec *ProjectLoggingSpec `json:"appliedSpec,omitempty" yaml:"appliedSpec,omitempty"`
	Conditions  []LoggingCondition  `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
