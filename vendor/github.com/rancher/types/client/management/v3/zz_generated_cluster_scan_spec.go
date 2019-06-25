package client

const (
	ClusterScanSpecType            = "clusterScanSpec"
	ClusterScanSpecFieldClusterID  = "clusterId"
	ClusterScanSpecFieldManual     = "manual"
	ClusterScanSpecFieldScanConfig = "scanConfig"
	ClusterScanSpecFieldScanType   = "scanType"
)

type ClusterScanSpec struct {
	ClusterID  string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Manual     bool               `json:"manual,omitempty" yaml:"manual,omitempty"`
	ScanConfig *ClusterScanConfig `json:"scanConfig,omitempty" yaml:"scanConfig,omitempty"`
	ScanType   string             `json:"scanType,omitempty" yaml:"scanType,omitempty"`
}
