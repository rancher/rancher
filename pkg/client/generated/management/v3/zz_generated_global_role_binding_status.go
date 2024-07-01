package client

const (
	GlobalRoleBindingStatusType                    = "globalRoleBindingStatus"
	GlobalRoleBindingStatusFieldConditions         = "conditions"
	GlobalRoleBindingStatusFieldLastUpdate         = "lastUpdateTime"
	GlobalRoleBindingStatusFieldObservedGeneration = "observedGeneration"
	GlobalRoleBindingStatusFieldSummary            = "summary"
)

type GlobalRoleBindingStatus struct {
	Conditions         []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	LastUpdate         string      `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Summary            string      `json:"summary,omitempty" yaml:"summary,omitempty"`
}
