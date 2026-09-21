package client

const (
	KeyCloakOIDCTestOutputType             = "keyCloakOIDCTestOutput"
	KeyCloakOIDCTestOutputFieldRedirectURL = "redirectUrl"
)

type KeyCloakOIDCTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
