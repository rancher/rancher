package client

const (
	NotifierStatusType                          = "notifierStatus"
	NotifierStatusFieldDingtalkCredentialSecret = "dingtalkCredentialSecret"
	NotifierStatusFieldSMTPCredentialSecret     = "smtpCredentialSecret"
	NotifierStatusFieldWechatCredentialSecret   = "wechatCredentialSecret"
)

type NotifierStatus struct {
	DingtalkCredentialSecret string `json:"dingtalkCredentialSecret,omitempty" yaml:"dingtalkCredentialSecret,omitempty"`
	SMTPCredentialSecret     string `json:"smtpCredentialSecret,omitempty" yaml:"smtpCredentialSecret,omitempty"`
	WechatCredentialSecret   string `json:"wechatCredentialSecret,omitempty" yaml:"wechatCredentialSecret,omitempty"`
}
