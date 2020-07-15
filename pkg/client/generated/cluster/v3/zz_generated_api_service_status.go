package client

const (
	APIServiceStatusType            = "apiServiceStatus"
	APIServiceStatusFieldConditions = "conditions"
)

type APIServiceStatus struct {
	Conditions []APIServiceCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
