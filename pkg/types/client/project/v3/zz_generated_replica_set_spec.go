package client

const (
	ReplicaSetSpecType                  = "replicaSetSpec"
	ReplicaSetSpecFieldReplicaSetConfig = "replicaSetConfig"
	ReplicaSetSpecFieldScale            = "scale"
	ReplicaSetSpecFieldSelector         = "selector"
	ReplicaSetSpecFieldTemplate         = "template"
)

type ReplicaSetSpec struct {
	ReplicaSetConfig *ReplicaSetConfig `json:"replicaSetConfig,omitempty" yaml:"replicaSetConfig,omitempty"`
	Scale            *int64            `json:"scale,omitempty" yaml:"scale,omitempty"`
	Selector         *LabelSelector    `json:"selector,omitempty" yaml:"selector,omitempty"`
	Template         *PodTemplateSpec  `json:"template,omitempty" yaml:"template,omitempty"`
}
