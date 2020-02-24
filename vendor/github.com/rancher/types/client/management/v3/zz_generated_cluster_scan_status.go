package client

const (
	ClusterScanStatusType               = "clusterScanStatus"
	ClusterScanStatusFieldCisScanStatus = "cisScanStatus"
	ClusterScanStatusFieldConditions    = "conditions"
)

type ClusterScanStatus struct {
	CisScanStatus *CisScanStatus         `json:"cisScanStatus,omitempty" yaml:"cisScanStatus,omitempty"`
	Conditions    []ClusterScanCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
