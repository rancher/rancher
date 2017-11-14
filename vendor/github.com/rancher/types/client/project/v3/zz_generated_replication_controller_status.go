package client

const (
	ReplicationControllerStatusType                      = "replicationControllerStatus"
	ReplicationControllerStatusFieldAvailableReplicas    = "availableReplicas"
	ReplicationControllerStatusFieldConditions           = "conditions"
	ReplicationControllerStatusFieldFullyLabeledReplicas = "fullyLabeledReplicas"
	ReplicationControllerStatusFieldObservedGeneration   = "observedGeneration"
	ReplicationControllerStatusFieldReadyReplicas        = "readyReplicas"
	ReplicationControllerStatusFieldReplicas             = "replicas"
)

type ReplicationControllerStatus struct {
	AvailableReplicas    *int64                           `json:"availableReplicas,omitempty"`
	Conditions           []ReplicationControllerCondition `json:"conditions,omitempty"`
	FullyLabeledReplicas *int64                           `json:"fullyLabeledReplicas,omitempty"`
	ObservedGeneration   *int64                           `json:"observedGeneration,omitempty"`
	ReadyReplicas        *int64                           `json:"readyReplicas,omitempty"`
	Replicas             *int64                           `json:"replicas,omitempty"`
}
