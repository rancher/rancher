package client

const (
	ReplicaSetSpecType                  = "replicaSetSpec"
	ReplicaSetSpecFieldReplicaSetConfig = "replicaSetConfig"
	ReplicaSetSpecFieldSelector         = "selector"
	ReplicaSetSpecFieldTemplate         = "template"
)

type ReplicaSetSpec struct {
	ReplicaSetConfig *ReplicaSetConfig `json:"replicaSetConfig,omitempty" yaml:"replicaSetConfig,omitempty"`
	Selector         *LabelSelector    `json:"selector,omitempty" yaml:"selector,omitempty"`
	Template         *PodTemplateSpec  `json:"template,omitempty" yaml:"template,omitempty"`
}
