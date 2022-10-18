package client

const (
	ProjectGroupSpecType                       = "projectGroupSpec"
	ProjectGroupSpecFieldDescription           = "description"
	ProjectGroupSpecFieldDisplayName           = "displayName"
	ProjectGroupSpecFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ProjectGroupSpecFieldGroupWaitSeconds      = "groupWaitSeconds"
	ProjectGroupSpecFieldProjectID             = "projectId"
	ProjectGroupSpecFieldRecipients            = "recipients"
	ProjectGroupSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
)

type ProjectGroupSpec struct {
	Description           string      `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName           string      `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	GroupIntervalSeconds  int64       `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64       `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	ProjectID             string      `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Recipients            []Recipient `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	RepeatIntervalSeconds int64       `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
}
