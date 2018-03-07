package client

const (
	PipelineSpecType                       = "pipelineSpec"
	PipelineSpecFieldDisplayName           = "displayName"
	PipelineSpecFieldStages                = "stages"
	PipelineSpecFieldTemplates             = "templates"
	PipelineSpecFieldTriggerCronExpression = "triggerCronExpression"
	PipelineSpecFieldTriggerCronTimezone   = "triggerCronTimezone"
	PipelineSpecFieldTriggerWebhook        = "triggerWebhook"
)

type PipelineSpec struct {
	DisplayName           string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Stages                []Stage           `json:"stages,omitempty" yaml:"stages,omitempty"`
	Templates             map[string]string `json:"templates,omitempty" yaml:"templates,omitempty"`
	TriggerCronExpression string            `json:"triggerCronExpression,omitempty" yaml:"triggerCronExpression,omitempty"`
	TriggerCronTimezone   string            `json:"triggerCronTimezone,omitempty" yaml:"triggerCronTimezone,omitempty"`
	TriggerWebhook        bool              `json:"triggerWebhook,omitempty" yaml:"triggerWebhook,omitempty"`
}
