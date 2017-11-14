package client

const (
	ReplicaSetStatusType                      = "replicaSetStatus"
	ReplicaSetStatusFieldAvailableReplicas    = "availableReplicas"
	ReplicaSetStatusFieldConditions           = "conditions"
	ReplicaSetStatusFieldFullyLabeledReplicas = "fullyLabeledReplicas"
	ReplicaSetStatusFieldObservedGeneration   = "observedGeneration"
	ReplicaSetStatusFieldReadyReplicas        = "readyReplicas"
	ReplicaSetStatusFieldReplicas             = "replicas"
)

type ReplicaSetStatus struct {
	AvailableReplicas    *int64                `json:"availableReplicas,omitempty"`
	Conditions           []ReplicaSetCondition `json:"conditions,omitempty"`
	FullyLabeledReplicas *int64                `json:"fullyLabeledReplicas,omitempty"`
	ObservedGeneration   *int64                `json:"observedGeneration,omitempty"`
	ReadyReplicas        *int64                `json:"readyReplicas,omitempty"`
	Replicas             *int64                `json:"replicas,omitempty"`
}
