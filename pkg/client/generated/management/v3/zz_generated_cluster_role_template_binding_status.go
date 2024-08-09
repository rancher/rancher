package client

const (
	ClusterRoleTemplateBindingStatusType                    = "clusterRoleTemplateBindingStatus"
	ClusterRoleTemplateBindingStatusFieldConditions         = "conditions"
	ClusterRoleTemplateBindingStatusFieldLastUpdate         = "lastUpdateTime"
	ClusterRoleTemplateBindingStatusFieldObservedGeneration = "observedGeneration"
	ClusterRoleTemplateBindingStatusFieldSummary            = "summary"
)

type ClusterRoleTemplateBindingStatus struct {
	Conditions         []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	LastUpdate         string      `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Summary            string      `json:"summary,omitempty" yaml:"summary,omitempty"`
}
