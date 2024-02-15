package client

const (
	GlobalRoleStatusType                    = "globalRoleStatus"
	GlobalRoleStatusFieldConditions         = "conditions"
	GlobalRoleStatusFieldLastUpdate         = "lastUpdateTime"
	GlobalRoleStatusFieldObservedGeneration = "observedGeneration"
	GlobalRoleStatusFieldSummary            = "summary"
)

type GlobalRoleStatus struct {
	Conditions         []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	LastUpdate         string      `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Summary            string      `json:"summary,omitempty" yaml:"summary,omitempty"`
}
