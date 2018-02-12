package client

const (
	NotifierSpecType                 = "notifierSpec"
	NotifierSpecFieldClusterId       = "clusterId"
	NotifierSpecFieldDescription     = "description"
	NotifierSpecFieldDisplayName     = "displayName"
	NotifierSpecFieldPagerdutyConfig = "pagerdutyConfig"
	NotifierSpecFieldSMTPConfig      = "smtpConfig"
	NotifierSpecFieldSlackConfig     = "slackConfig"
	NotifierSpecFieldWebhookConfig   = "webhookConfig"
)

type NotifierSpec struct {
	ClusterId       string           `json:"clusterId,omitempty"`
	Description     string           `json:"description,omitempty"`
	DisplayName     string           `json:"displayName,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty"`
}
