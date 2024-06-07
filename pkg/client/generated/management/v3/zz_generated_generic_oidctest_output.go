package client

const (
	GenericOIDCTestOutputType             = "genericOIDCTestOutput"
	GenericOIDCTestOutputFieldRedirectURL = "redirectUrl"
)

type GenericOIDCTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
