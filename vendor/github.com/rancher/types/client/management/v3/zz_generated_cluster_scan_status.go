package client

const (
	ClusterScanStatusType            = "clusterScanStatus"
	ClusterScanStatusFieldConditions = "conditions"
)

type ClusterScanStatus struct {
	Conditions []ClusterScanCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
