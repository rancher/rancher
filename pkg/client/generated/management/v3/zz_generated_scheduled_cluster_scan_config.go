package client

const (
	ScheduledClusterScanConfigType              = "scheduledClusterScanConfig"
	ScheduledClusterScanConfigFieldCronSchedule = "cronSchedule"
	ScheduledClusterScanConfigFieldRetention    = "retention"
)

type ScheduledClusterScanConfig struct {
	CronSchedule string `json:"cronSchedule,omitempty" yaml:"cronSchedule,omitempty"`
	Retention    int64  `json:"retention,omitempty" yaml:"retention,omitempty"`
}
