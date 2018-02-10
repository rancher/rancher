package client

const (
	JobTemplateSpecType            = "jobTemplateSpec"
	JobTemplateSpecFieldObjectMeta = "metadata"
	JobTemplateSpecFieldSpec       = "spec"
)

type JobTemplateSpec struct {
	ObjectMeta *ObjectMeta `json:"metadata,omitempty"`
	Spec       *JobSpec    `json:"spec,omitempty"`
}
