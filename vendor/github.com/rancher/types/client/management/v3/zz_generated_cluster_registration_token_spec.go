package client

const (
	ClusterRegistrationTokenSpecType           = "clusterRegistrationTokenSpec"
	ClusterRegistrationTokenSpecFieldClusterId = "clusterId"
)

type ClusterRegistrationTokenSpec struct {
	ClusterId string `json:"clusterId,omitempty"`
}
