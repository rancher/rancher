package client

const (
	SMTPConfigType                  = "smtpConfig"
	SMTPConfigFieldDefaultRecipient = "defaultRecipient"
	SMTPConfigFieldHost             = "host"
	SMTPConfigFieldPassword         = "password"
	SMTPConfigFieldPort             = "port"
	SMTPConfigFieldSender           = "sender"
	SMTPConfigFieldTLS              = "tls"
	SMTPConfigFieldUsername         = "username"
)

type SMTPConfig struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty" yaml:"defaultRecipient,omitempty"`
	Host             string `json:"host,omitempty" yaml:"host,omitempty"`
	Password         string `json:"password,omitempty" yaml:"password,omitempty"`
	Port             int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Sender           string `json:"sender,omitempty" yaml:"sender,omitempty"`
	TLS              *bool  `json:"tls,omitempty" yaml:"tls,omitempty"`
	Username         string `json:"username,omitempty" yaml:"username,omitempty"`
}
