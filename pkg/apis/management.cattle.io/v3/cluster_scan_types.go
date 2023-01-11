package v3

type ClusterScanRunType string

const (
	ClusterScanRunTypeManual    ClusterScanRunType = "manual"
	ClusterScanRunTypeScheduled ClusterScanRunType = "scheduled"
)
