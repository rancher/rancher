package client

const (
	ClusterScanConfigType               = "clusterScanConfig"
	ClusterScanConfigFieldCisScanConfig = "cisScanConfig"
)

type ClusterScanConfig struct {
	CisScanConfig *CisScanConfig `json:"cisScanConfig,omitempty" yaml:"cisScanConfig,omitempty"`
}
