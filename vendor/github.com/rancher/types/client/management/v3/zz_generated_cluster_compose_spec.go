package client

const (
	ClusterComposeSpecType                = "clusterComposeSpec"
	ClusterComposeSpecFieldClusterId      = "clusterId"
	ClusterComposeSpecFieldRancherCompose = "rancherCompose"
)

type ClusterComposeSpec struct {
	ClusterId      string `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	RancherCompose string `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
}
