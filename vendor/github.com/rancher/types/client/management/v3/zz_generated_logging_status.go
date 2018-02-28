package client

const (
	LoggingStatusType            = "loggingStatus"
	LoggingStatusFieldConditions = "conditions"
)

type LoggingStatus struct {
	Conditions []LoggingCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
