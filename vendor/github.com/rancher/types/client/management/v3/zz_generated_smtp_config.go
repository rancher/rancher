package client

const (
	SMTPConfigType                  = "smtpConfig"
	SMTPConfigFieldDefaultRecipient = "defaultRecipient"
	SMTPConfigFieldHost             = "host"
	SMTPConfigFieldPassword         = "password"
	SMTPConfigFieldPort             = "port"
	SMTPConfigFieldTLS              = "tls"
	SMTPConfigFieldUsername         = "username"
)

type SMTPConfig struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty"`
	Host             string `json:"host,omitempty"`
	Password         string `json:"password,omitempty"`
	Port             *int64 `json:"port,omitempty"`
	TLS              bool   `json:"tls,omitempty"`
	Username         string `json:"username,omitempty"`
}
