package client

const (
	AzureADConfigApplyInputType        = "azureADConfigApplyInput"
	AzureADConfigApplyInputFieldCode   = "code"
	AzureADConfigApplyInputFieldConfig = "config"
)

type AzureADConfigApplyInput struct {
	Code   string         `json:"code,omitempty" yaml:"code,omitempty"`
	Config *AzureADConfig `json:"config,omitempty" yaml:"config,omitempty"`
}
