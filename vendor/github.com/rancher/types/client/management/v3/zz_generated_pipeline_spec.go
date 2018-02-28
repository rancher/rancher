package client

const (
	PipelineSpecType                       = "pipelineSpec"
	PipelineSpecFieldDisplayName           = "displayName"
	PipelineSpecFieldProjectId             = "projectId"
	PipelineSpecFieldStages                = "stages"
	PipelineSpecFieldTriggerCronExpression = "triggerCronExpression"
	PipelineSpecFieldTriggerCronTimezone   = "triggerCronTimezone"
	PipelineSpecFieldTriggerWebhook        = "triggerWebhook"
)

type PipelineSpec struct {
	DisplayName           string  `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	ProjectId             string  `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Stages                []Stage `json:"stages,omitempty" yaml:"stages,omitempty"`
	TriggerCronExpression string  `json:"triggerCronExpression,omitempty" yaml:"triggerCronExpression,omitempty"`
	TriggerCronTimezone   string  `json:"triggerCronTimezone,omitempty" yaml:"triggerCronTimezone,omitempty"`
	TriggerWebhook        bool    `json:"triggerWebhook,omitempty" yaml:"triggerWebhook,omitempty"`
}
