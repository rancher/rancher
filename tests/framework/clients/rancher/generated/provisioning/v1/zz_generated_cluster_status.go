package client

const (
	ClusterStatusType                    = "clusterStatus"
	ClusterStatusFieldAgentDeployed      = "agentDeployed"
	ClusterStatusFieldClientSecretName   = "clientSecretName"
	ClusterStatusFieldClusterName        = "clusterName"
	ClusterStatusFieldConditions         = "conditions"
	ClusterStatusFieldObservedGeneration = "observedGeneration"
	ClusterStatusFieldReady              = "ready"
)

type ClusterStatus struct {
	AgentDeployed      bool               `json:"agentDeployed,omitempty" yaml:"agentDeployed,omitempty"`
	ClientSecretName   string             `json:"clientSecretName,omitempty" yaml:"clientSecretName,omitempty"`
	ClusterName        string             `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	Conditions         []GenericCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Ready              bool               `json:"ready,omitempty" yaml:"ready,omitempty"`
}
