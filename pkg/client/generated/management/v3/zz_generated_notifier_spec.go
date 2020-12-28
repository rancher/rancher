package client

const (
	NotifierSpecType                 = "notifierSpec"
	NotifierSpecFieldClusterID       = "clusterId"
	NotifierSpecFieldDescription     = "description"
	NotifierSpecFieldDingtalkConfig  = "dingtalkConfig"
	NotifierSpecFieldDisplayName     = "displayName"
	NotifierSpecFieldMSTeamsConfig   = "msteamsConfig"
	NotifierSpecFieldPagerdutyConfig = "pagerdutyConfig"
	NotifierSpecFieldSMTPConfig      = "smtpConfig"
	NotifierSpecFieldSendResolved    = "sendResolved"
	NotifierSpecFieldSlackConfig     = "slackConfig"
	NotifierSpecFieldWebhookConfig   = "webhookConfig"
	NotifierSpecFieldWechatConfig    = "wechatConfig"
)

type NotifierSpec struct {
	ClusterID       string           `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Description     string           `json:"description,omitempty" yaml:"description,omitempty"`
	DingtalkConfig  *DingtalkConfig  `json:"dingtalkConfig,omitempty" yaml:"dingtalkConfig,omitempty"`
	DisplayName     string           `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	MSTeamsConfig   *MSTeamsConfig   `json:"msteamsConfig,omitempty" yaml:"msteamsConfig,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty" yaml:"pagerdutyConfig,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty" yaml:"smtpConfig,omitempty"`
	SendResolved    bool             `json:"sendResolved,omitempty" yaml:"sendResolved,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty" yaml:"slackConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty" yaml:"webhookConfig,omitempty"`
	WechatConfig    *WechatConfig    `json:"wechatConfig,omitempty" yaml:"wechatConfig,omitempty"`
}
