package client

const (
	CronJobConfigType                            = "cronJobConfig"
	CronJobConfigFieldConcurrencyPolicy          = "concurrencyPolicy"
	CronJobConfigFieldFailedJobsHistoryLimit     = "failedJobsHistoryLimit"
	CronJobConfigFieldJobTemplate                = "jobTemplate"
	CronJobConfigFieldSchedule                   = "schedule"
	CronJobConfigFieldStartingDeadlineSeconds    = "startingDeadlineSeconds"
	CronJobConfigFieldSuccessfulJobsHistoryLimit = "successfulJobsHistoryLimit"
	CronJobConfigFieldSuspend                    = "suspend"
)

type CronJobConfig struct {
	ConcurrencyPolicy          string           `json:"concurrencyPolicy,omitempty"`
	FailedJobsHistoryLimit     *int64           `json:"failedJobsHistoryLimit,omitempty"`
	JobTemplate                *JobTemplateSpec `json:"jobTemplate,omitempty"`
	Schedule                   string           `json:"schedule,omitempty"`
	StartingDeadlineSeconds    *int64           `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit *int64           `json:"successfulJobsHistoryLimit,omitempty"`
	Suspend                    *bool            `json:"suspend,omitempty"`
}
