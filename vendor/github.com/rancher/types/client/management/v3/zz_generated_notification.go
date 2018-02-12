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
	Message         string           `json:"message,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty"`
}
