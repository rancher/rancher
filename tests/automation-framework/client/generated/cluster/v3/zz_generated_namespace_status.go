package client

const (
	NamespaceStatusType            = "namespaceStatus"
	NamespaceStatusFieldConditions = "conditions"
	NamespaceStatusFieldPhase      = "phase"
)

type NamespaceStatus struct {
	Conditions []NamespaceCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Phase      string               `json:"phase,omitempty" yaml:"phase,omitempty"`
}
