package client

const (
	AuthConfigStatusType            = "authConfigStatus"
	AuthConfigStatusFieldConditions = "conditions"
)

type AuthConfigStatus struct {
	Conditions []AuthConfigConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
