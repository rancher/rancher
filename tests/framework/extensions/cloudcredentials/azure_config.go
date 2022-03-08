package cloudcredentials

const AzureCredentialConfigurationFileKey = "azureCredentials"

type AzureCredentialConfig struct {
	ClientID       string `json:"clientId" yaml:"clientId"`
	ClientSecret   string `json:"clientSecret" yaml:"clientSecret"`
	SubscriptionID string `json:"subscriptionId" yaml:"subscriptionId"`
	Environment    string `json:"environment" yaml:"environment"`
}
