package client

const (
	ReplicaSetSpecType                  = "replicaSetSpec"
	ReplicaSetSpecFieldReplicaSetConfig = "replicaSetConfig"
	ReplicaSetSpecFieldSelector         = "selector"
	ReplicaSetSpecFieldTemplate         = "template"
)

type ReplicaSetSpec struct {
	ReplicaSetConfig *ReplicaSetConfig `json:"replicaSetConfig,omitempty"`
	Selector         *LabelSelector    `json:"selector,omitempty"`
	Template         *PodTemplateSpec  `json:"template,omitempty"`
}
