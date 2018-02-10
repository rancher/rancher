package client

const (
	ReplicaSetSpecType            = "replicaSetSpec"
	ReplicaSetSpecFieldReplicaSet = "replicaSet"
	ReplicaSetSpecFieldSelector   = "selector"
	ReplicaSetSpecFieldTemplate   = "template"
)

type ReplicaSetSpec struct {
	ReplicaSet *ReplicaSetConfig `json:"replicaSet,omitempty"`
	Selector   *LabelSelector    `json:"selector,omitempty"`
	Template   *PodTemplateSpec  `json:"template,omitempty"`
}
