package client

const (
	ScheduledClusterScanStatusType                  = "scheduledClusterScanStatus"
	ScheduledClusterScanStatusFieldEnabled          = "enabled"
	ScheduledClusterScanStatusFieldLastRunTimestamp = "lastRunTimestamp"
)

type ScheduledClusterScanStatus struct {
	Enabled          bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	LastRunTimestamp string `json:"lastRunTimestamp,omitempty" yaml:"lastRunTimestamp,omitempty"`
}
