package client

const (
	ClusterScanRuleType              = "clusterScanRule"
	ClusterScanRuleFieldFailuresOnly = "failuresOnly"
	ClusterScanRuleFieldScanRunType  = "scanRunType"
)

type ClusterScanRule struct {
	FailuresOnly bool   `json:"failuresOnly,omitempty" yaml:"failuresOnly,omitempty"`
	ScanRunType  string `json:"scanRunType,omitempty" yaml:"scanRunType,omitempty"`
}
