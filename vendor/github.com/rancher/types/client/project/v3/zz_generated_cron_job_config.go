package client

const (
	CronJobConfigType                            = "cronJobConfig"
	CronJobConfigFieldConcurrencyPolicy          = "concurrencyPolicy"
	CronJobConfigFieldFailedJobsHistoryLimit     = "failedJobsHistoryLimit"
	CronJobConfigFieldJobAnnotations             = "jobAnnotations"
	CronJobConfigFieldJobConfig                  = "jobConfig"
	CronJobConfigFieldJobLabels                  = "jobLabels"
	CronJobConfigFieldSchedule                   = "schedule"
	CronJobConfigFieldStartingDeadlineSeconds    = "startingDeadlineSeconds"
	CronJobConfigFieldSuccessfulJobsHistoryLimit = "successfulJobsHistoryLimit"
	CronJobConfigFieldSuspend                    = "suspend"
)

type CronJobConfig struct {
	ConcurrencyPolicy          string            `json:"concurrencyPolicy,omitempty"`
	FailedJobsHistoryLimit     *int64            `json:"failedJobsHistoryLimit,omitempty"`
	JobAnnotations             map[string]string `json:"jobAnnotations,omitempty"`
	JobConfig                  *JobConfig        `json:"jobConfig,omitempty"`
	JobLabels                  map[string]string `json:"jobLabels,omitempty"`
	Schedule                   string            `json:"schedule,omitempty"`
	StartingDeadlineSeconds    *int64            `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit *int64            `json:"successfulJobsHistoryLimit,omitempty"`
	Suspend                    *bool             `json:"suspend,omitempty"`
}
