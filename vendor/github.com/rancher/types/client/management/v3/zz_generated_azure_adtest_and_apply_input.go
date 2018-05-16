package client

const (
	AzureADTestAndApplyInputType               = "azureADTestAndApplyInput"
	AzureADTestAndApplyInputFieldAzureADConfig = "azureAdConfig"
	AzureADTestAndApplyInputFieldEnabled       = "enabled"
	AzureADTestAndApplyInputFieldPassword      = "password"
	AzureADTestAndApplyInputFieldUsername      = "username"
)

type AzureADTestAndApplyInput struct {
	AzureADConfig *AzureADConfig `json:"azureAdConfig,omitempty" yaml:"azureAdConfig,omitempty"`
	Enabled       bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Password      string         `json:"password,omitempty" yaml:"password,omitempty"`
	Username      string         `json:"username,omitempty" yaml:"username,omitempty"`
}
