package client

const (
	AuthAppInputType                = "authAppInput"
	AuthAppInputFieldClientID       = "clientId"
	AuthAppInputFieldClientSecret   = "clientSecret"
	AuthAppInputFieldCode           = "code"
	AuthAppInputFieldHost           = "host"
	AuthAppInputFieldRedirectURL    = "redirectUrl"
	AuthAppInputFieldSourceCodeType = "sourceCodeType"
	AuthAppInputFieldTLS            = "tls"
)

type AuthAppInput struct {
	ClientID       string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Code           string `json:"code,omitempty" yaml:"code,omitempty"`
	Host           string `json:"host,omitempty" yaml:"host,omitempty"`
	RedirectURL    string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	TLS            bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
}
