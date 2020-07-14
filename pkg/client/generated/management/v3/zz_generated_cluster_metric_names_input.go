package client

const (
	ClusterMetricNamesInputType             = "clusterMetricNamesInput"
	ClusterMetricNamesInputFieldClusterName = "clusterId"
)

type ClusterMetricNamesInput struct {
	ClusterName string `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
}
