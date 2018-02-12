package client

const (
	CronJobSpecType         = "cronJobSpec"
	CronJobSpecFieldCronJob = "cronJob"
)

type CronJobSpec struct {
	CronJob *CronJobConfig `json:"cronJob,omitempty"`
}
