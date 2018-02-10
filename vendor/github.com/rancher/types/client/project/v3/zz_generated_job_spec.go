package client

const (
	JobSpecType                       = "jobSpec"
	JobSpecFieldActiveDeadlineSeconds = "activeDeadlineSeconds"
	JobSpecFieldBackoffLimit          = "backoffLimit"
	JobSpecFieldCompletions           = "completions"
	JobSpecFieldManualSelector        = "manualSelector"
	JobSpecFieldParallelism           = "parallelism"
	JobSpecFieldSelector              = "selector"
	JobSpecFieldTemplate              = "template"
)

type JobSpec struct {
	ActiveDeadlineSeconds *int64           `json:"activeDeadlineSeconds,omitempty"`
	BackoffLimit          *int64           `json:"backoffLimit,omitempty"`
	Completions           *int64           `json:"completions,omitempty"`
	ManualSelector        *bool            `json:"manualSelector,omitempty"`
	Parallelism           *int64           `json:"parallelism,omitempty"`
	Selector              *LabelSelector   `json:"selector,omitempty"`
	Template              *PodTemplateSpec `json:"template,omitempty"`
}
