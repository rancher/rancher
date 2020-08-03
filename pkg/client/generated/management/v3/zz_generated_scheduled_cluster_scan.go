package client

const (
	ScheduledClusterScanType                = "scheduledClusterScan"
	ScheduledClusterScanFieldEnabled        = "enabled"
	ScheduledClusterScanFieldScanConfig     = "scanConfig"
	ScheduledClusterScanFieldScheduleConfig = "scheduleConfig"
)

type ScheduledClusterScan struct {
	Enabled        bool                        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ScanConfig     *ClusterScanConfig          `json:"scanConfig,omitempty" yaml:"scanConfig,omitempty"`
	ScheduleConfig *ScheduledClusterScanConfig `json:"scheduleConfig,omitempty" yaml:"scheduleConfig,omitempty"`
}
