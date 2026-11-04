package client

const (
	AzureADConfigApplyInputType            = "azureADConfigApplyInput"
	AzureADConfigApplyInputFieldCode       = "code"
	AzureADConfigApplyInputFieldConfig     = "config"
	AzureADConfigApplyInputFieldConfigName = "configName"
)

type AzureADConfigApplyInput struct {
	Code       string         `json:"code,omitempty" yaml:"code,omitempty"`
	Config     *AzureADConfig `json:"config,omitempty" yaml:"config,omitempty"`
	ConfigName string         `json:"configName,omitempty" yaml:"configName,omitempty"`
}
