package client

const (
	ClusterRoleTemplateBindingStatusType                          = "clusterRoleTemplateBindingStatus"
	ClusterRoleTemplateBindingStatusFieldLastUpdateTime           = "lastUpdateTime"
	ClusterRoleTemplateBindingStatusFieldLocalConditions          = "localConditions"
	ClusterRoleTemplateBindingStatusFieldObservedGenerationLocal  = "observedGenerationLocal"
	ClusterRoleTemplateBindingStatusFieldObservedGenerationRemote = "observedGenerationRemote"
	ClusterRoleTemplateBindingStatusFieldRemoteConditions         = "remoteConditions"
	ClusterRoleTemplateBindingStatusFieldSummary                  = "summary"
	ClusterRoleTemplateBindingStatusFieldSummaryLocal             = "summaryLocal"
	ClusterRoleTemplateBindingStatusFieldSummaryRemote            = "summaryRemote"
)

type ClusterRoleTemplateBindingStatus struct {
	LastUpdateTime           string      `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	LocalConditions          []Condition `json:"localConditions,omitempty" yaml:"localConditions,omitempty"`
	ObservedGenerationLocal  int64       `json:"observedGenerationLocal,omitempty" yaml:"observedGenerationLocal,omitempty"`
	ObservedGenerationRemote int64       `json:"observedGenerationRemote,omitempty" yaml:"observedGenerationRemote,omitempty"`
	RemoteConditions         []Condition `json:"remoteConditions,omitempty" yaml:"remoteConditions,omitempty"`
	Summary                  string      `json:"summary,omitempty" yaml:"summary,omitempty"`
	SummaryLocal             string      `json:"summaryLocal,omitempty" yaml:"summaryLocal,omitempty"`
	SummaryRemote            string      `json:"summaryRemote,omitempty" yaml:"summaryRemote,omitempty"`
}
