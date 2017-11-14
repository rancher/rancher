package client

const (
	StatefulSetStatusType                    = "statefulSetStatus"
	StatefulSetStatusFieldCollisionCount     = "collisionCount"
	StatefulSetStatusFieldCurrentReplicas    = "currentReplicas"
	StatefulSetStatusFieldCurrentRevision    = "currentRevision"
	StatefulSetStatusFieldObservedGeneration = "observedGeneration"
	StatefulSetStatusFieldReadyReplicas      = "readyReplicas"
	StatefulSetStatusFieldReplicas           = "replicas"
	StatefulSetStatusFieldUpdateRevision     = "updateRevision"
	StatefulSetStatusFieldUpdatedReplicas    = "updatedReplicas"
)

type StatefulSetStatus struct {
	CollisionCount     *int64 `json:"collisionCount,omitempty"`
	CurrentReplicas    *int64 `json:"currentReplicas,omitempty"`
	CurrentRevision    string `json:"currentRevision,omitempty"`
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	ReadyReplicas      *int64 `json:"readyReplicas,omitempty"`
	Replicas           *int64 `json:"replicas,omitempty"`
	UpdateRevision     string `json:"updateRevision,omitempty"`
	UpdatedReplicas    *int64 `json:"updatedReplicas,omitempty"`
}
