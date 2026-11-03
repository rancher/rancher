package client

const (
	ClusterRegistrationTokenSpecType             = "clusterRegistrationTokenSpec"
	ClusterRegistrationTokenSpecFieldClusterID   = "clusterId"
	ClusterRegistrationTokenSpecFieldGracePeriod = "gracePeriod"
	ClusterRegistrationTokenSpecFieldTTL         = "ttl"
)

type ClusterRegistrationTokenSpec struct {
	ClusterID   string `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	GracePeriod *int64 `json:"gracePeriod,omitempty" yaml:"gracePeriod,omitempty"`
	TTL         *int64 `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}
