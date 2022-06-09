package client

const (
	WebSpecType           = "webSpec"
	WebSpecFieldPageTitle = "pageTitle"
	WebSpecFieldTLSConfig = "tlsConfig"
)

type WebSpec struct {
	PageTitle string        `json:"pageTitle,omitempty" yaml:"pageTitle,omitempty"`
	TLSConfig *WebTLSConfig `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}
