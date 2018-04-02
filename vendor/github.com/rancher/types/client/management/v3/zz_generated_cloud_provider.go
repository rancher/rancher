package client

const (
	CloudProviderType                    = "cloudProvider"
	CloudProviderFieldAWSCloudProvider   = "awsCloudProvider"
	CloudProviderFieldAzureCloudProvider = "azureCloudProvider"
	CloudProviderFieldCloudConfig        = "cloudConfig"
	CloudProviderFieldName               = "name"
)

type CloudProvider struct {
	AWSCloudProvider   *AWSCloudProvider   `json:"awsCloudProvider,omitempty" yaml:"awsCloudProvider,omitempty"`
	AzureCloudProvider *AzureCloudProvider `json:"azureCloudProvider,omitempty" yaml:"azureCloudProvider,omitempty"`
	CloudConfig        map[string]string   `json:"cloudConfig,omitempty" yaml:"cloudConfig,omitempty"`
	Name               string              `json:"name,omitempty" yaml:"name,omitempty"`
}
