package client

const (
	NotificationType                 = "notification"
	NotificationFieldMessage         = "message"
	NotificationFieldPagerdutyConfig = "pagerdutyConfig"
	NotificationFieldSMTPConfig      = "smtpConfig"
	NotificationFieldSlackConfig     = "slackConfig"
	NotificationFieldWebhookConfig   = "webhookConfig"
)

type Notification struct {
	Message         string           `json:"message,omitempty" yaml:"message,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty" yaml:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty" yaml:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty" yaml:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty" yaml:"webhookConfig,omitempty"`
}
