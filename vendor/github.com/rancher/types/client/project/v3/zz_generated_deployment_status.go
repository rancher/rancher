package client

const (
	DeploymentStatusType                     = "deploymentStatus"
	DeploymentStatusFieldAvailableReplicas   = "availableReplicas"
	DeploymentStatusFieldCollisionCount      = "collisionCount"
	DeploymentStatusFieldConditions          = "conditions"
	DeploymentStatusFieldObservedGeneration  = "observedGeneration"
	DeploymentStatusFieldReadyReplicas       = "readyReplicas"
	DeploymentStatusFieldReplicas            = "replicas"
	DeploymentStatusFieldUnavailableReplicas = "unavailableReplicas"
	DeploymentStatusFieldUpdatedReplicas     = "updatedReplicas"
)

type DeploymentStatus struct {
	AvailableReplicas   *int64                `json:"availableReplicas,omitempty"`
	CollisionCount      *int64                `json:"collisionCount,omitempty"`
	Conditions          []DeploymentCondition `json:"conditions,omitempty"`
	ObservedGeneration  *int64                `json:"observedGeneration,omitempty"`
	ReadyReplicas       *int64                `json:"readyReplicas,omitempty"`
	Replicas            *int64                `json:"replicas,omitempty"`
	UnavailableReplicas *int64                `json:"unavailableReplicas,omitempty"`
	UpdatedReplicas     *int64                `json:"updatedReplicas,omitempty"`
}
