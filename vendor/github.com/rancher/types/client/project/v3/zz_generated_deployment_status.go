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
	AvailableReplicas   int64                 `json:"availableReplicas,omitempty" yaml:"availableReplicas,omitempty"`
	CollisionCount      *int64                `json:"collisionCount,omitempty" yaml:"collisionCount,omitempty"`
	Conditions          []DeploymentCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ObservedGeneration  int64                 `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	ReadyReplicas       int64                 `json:"readyReplicas,omitempty" yaml:"readyReplicas,omitempty"`
	Replicas            int64                 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	UnavailableReplicas int64                 `json:"unavailableReplicas,omitempty" yaml:"unavailableReplicas,omitempty"`
	UpdatedReplicas     int64                 `json:"updatedReplicas,omitempty" yaml:"updatedReplicas,omitempty"`
}
