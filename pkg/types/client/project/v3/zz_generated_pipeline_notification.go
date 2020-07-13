package client

const (
	PipelineNotificationType            = "pipelineNotification"
	PipelineNotificationFieldCondition  = "condition"
	PipelineNotificationFieldMessage    = "message"
	PipelineNotificationFieldRecipients = "recipients"
)

type PipelineNotification struct {
	Condition  []string    `json:"condition,omitempty" yaml:"condition,omitempty"`
	Message    string      `json:"message,omitempty" yaml:"message,omitempty"`
	Recipients []Recipient `json:"recipients,omitempty" yaml:"recipients,omitempty"`
}
