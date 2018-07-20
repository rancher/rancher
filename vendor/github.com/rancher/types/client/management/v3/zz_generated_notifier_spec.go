package client

const (
	NotifierSpecType                 = "notifierSpec"
	NotifierSpecFieldClusterID       = "clusterId"
	NotifierSpecFieldDescription     = "description"
	NotifierSpecFieldDisplayName     = "displayName"
	NotifierSpecFieldPagerdutyConfig = "pagerdutyConfig"
	NotifierSpecFieldSMTPConfig      = "smtpConfig"
	NotifierSpecFieldSlackConfig     = "slackConfig"
	NotifierSpecFieldWebhookConfig   = "webhookConfig"
)

type NotifierSpec struct {
	ClusterID       string           `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Description     string           `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName     string           `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty" yaml:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty" yaml:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty" yaml:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty" yaml:"webhookConfig,omitempty"`
}
