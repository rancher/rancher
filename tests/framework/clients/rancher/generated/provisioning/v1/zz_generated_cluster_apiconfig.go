package client

const (
	ClusterAPIConfigType             = "clusterAPIConfig"
	ClusterAPIConfigFieldClusterName = "clusterName"
)

type ClusterAPIConfig struct {
	ClusterName string `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
}
