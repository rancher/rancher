package client

const (
	ClusterScanSpecType            = "clusterScanSpec"
	ClusterScanSpecFieldClusterID  = "clusterId"
	ClusterScanSpecFieldRunType    = "runType"
	ClusterScanSpecFieldScanConfig = "scanConfig"
	ClusterScanSpecFieldScanType   = "scanType"
)

type ClusterScanSpec struct {
	ClusterID  string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	RunType    string             `json:"runType,omitempty" yaml:"runType,omitempty"`
	ScanConfig *ClusterScanConfig `json:"scanConfig,omitempty" yaml:"scanConfig,omitempty"`
	ScanType   string             `json:"scanType,omitempty" yaml:"scanType,omitempty"`
}
