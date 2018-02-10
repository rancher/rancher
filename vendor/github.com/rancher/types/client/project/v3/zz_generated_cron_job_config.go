package client

const (
	CronJobConfigType                            = "cronJobConfig"
	CronJobConfigFieldActiveDeadlineSeconds      = "activeDeadlineSeconds"
	CronJobConfigFieldBackoffLimit               = "backoffLimit"
	CronJobConfigFieldCompletions                = "completions"
	CronJobConfigFieldConcurrencyPolicy          = "concurrencyPolicy"
	CronJobConfigFieldFailedJobsHistoryLimit     = "failedJobsHistoryLimit"
	CronJobConfigFieldManualSelector             = "manualSelector"
	CronJobConfigFieldParallelism                = "parallelism"
	CronJobConfigFieldSchedule                   = "schedule"
	CronJobConfigFieldStartingDeadlineSeconds    = "startingDeadlineSeconds"
	CronJobConfigFieldSuccessfulJobsHistoryLimit = "successfulJobsHistoryLimit"
	CronJobConfigFieldSuspend                    = "suspend"
)

type CronJobConfig struct {
	ActiveDeadlineSeconds      *int64 `json:"activeDeadlineSeconds,omitempty"`
	BackoffLimit               *int64 `json:"backoffLimit,omitempty"`
	Completions                *int64 `json:"completions,omitempty"`
	ConcurrencyPolicy          string `json:"concurrencyPolicy,omitempty"`
	FailedJobsHistoryLimit     *int64 `json:"failedJobsHistoryLimit,omitempty"`
	ManualSelector             *bool  `json:"manualSelector,omitempty"`
	Parallelism                *int64 `json:"parallelism,omitempty"`
	Schedule                   string `json:"schedule,omitempty"`
	StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit *int64 `json:"successfulJobsHistoryLimit,omitempty"`
	Suspend                    *bool  `json:"suspend,omitempty"`
}
