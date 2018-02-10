package client

const (
	CronJobStatusType                  = "cronJobStatus"
	CronJobStatusFieldActive           = "active"
	CronJobStatusFieldLastScheduleTime = "lastScheduleTime"
)

type CronJobStatus struct {
	Active           []ObjectReference `json:"active,omitempty"`
	LastScheduleTime string            `json:"lastScheduleTime,omitempty"`
}
