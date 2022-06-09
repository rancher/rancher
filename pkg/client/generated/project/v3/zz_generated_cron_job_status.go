package client

const (
	CronJobStatusType                    = "cronJobStatus"
	CronJobStatusFieldActive             = "active"
	CronJobStatusFieldLastScheduleTime   = "lastScheduleTime"
	CronJobStatusFieldLastSuccessfulTime = "lastSuccessfulTime"
)

type CronJobStatus struct {
	Active             []ObjectReference `json:"active,omitempty" yaml:"active,omitempty"`
	LastScheduleTime   string            `json:"lastScheduleTime,omitempty" yaml:"lastScheduleTime,omitempty"`
	LastSuccessfulTime string            `json:"lastSuccessfulTime,omitempty" yaml:"lastSuccessfulTime,omitempty"`
}
