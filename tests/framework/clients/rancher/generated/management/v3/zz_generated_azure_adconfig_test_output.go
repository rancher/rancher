package client

const (
	AzureADConfigTestOutputType             = "azureADConfigTestOutput"
	AzureADConfigTestOutputFieldRedirectURL = "redirectUrl"
)

type AzureADConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl,omitempty" yaml:"redirectUrl,omitempty"`
}
