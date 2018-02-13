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
	DisplayName           string  `json:"displayName,omitempty"`
	ProjectId             string  `json:"projectId,omitempty"`
	Stages                []Stage `json:"stages,omitempty"`
	TriggerCronExpression string  `json:"triggerCronExpression,omitempty"`
	TriggerCronTimezone   string  `json:"triggerCronTimezone,omitempty"`
	TriggerWebhook        bool    `json:"triggerWebhook,omitempty"`
}
