package client

const (
	NotificationType                 = "notification"
	NotificationFieldMessage         = "message"
	NotificationFieldOpsgenieConfig  = "opsgenieConfig"
	NotificationFieldPagerdutyConfig = "pagerdutyConfig"
	NotificationFieldSMTPConfig      = "smtpConfig"
	NotificationFieldSlackConfig     = "slackConfig"
	NotificationFieldWebhookConfig   = "webhookConfig"
	NotificationFieldWechatConfig    = "wechatConfig"
)

type Notification struct {
	Message         string           `json:"message,omitempty" yaml:"message,omitempty"`
	OpsgenieConfig  *OpsgenieConfig  `json:"opsgenieConfig,omitempty" yaml:"opsgenieConfig,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty" yaml:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty" yaml:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty" yaml:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty" yaml:"webhookConfig,omitempty"`
	WechatConfig    *WechatConfig    `json:"wechatConfig,omitempty" yaml:"wechatConfig,omitempty"`
}
