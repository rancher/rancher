package cloudcredentials

// The json/yaml config key for the azure cloud credential config
const AzureCredentialConfigurationFileKey = "azureCredentials"

// AzureCredentialConfig is configuration need to create an azure cloud credential
type AzureCredentialConfig struct {
	ClientID       string `json:"clientId" yaml:"clientId"`
	ClientSecret   string `json:"clientSecret" yaml:"clientSecret"`
	SubscriptionID string `json:"subscriptionId" yaml:"subscriptionId"`
	Environment    string `json:"environment" yaml:"environment"`
}
