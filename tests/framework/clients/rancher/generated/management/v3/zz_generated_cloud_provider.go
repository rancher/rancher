package client

const (
	CloudProviderType                        = "cloudProvider"
	CloudProviderFieldAWSCloudProvider       = "awsCloudProvider"
	CloudProviderFieldAzureCloudProvider     = "azureCloudProvider"
	CloudProviderFieldCustomCloudProvider    = "customCloudProvider"
	CloudProviderFieldName                   = "name"
	CloudProviderFieldOpenstackCloudProvider = "openstackCloudProvider"
	CloudProviderFieldVsphereCloudProvider   = "vsphereCloudProvider"
)

type CloudProvider struct {
	AWSCloudProvider       *AWSCloudProvider       `json:"awsCloudProvider,omitempty" yaml:"awsCloudProvider,omitempty"`
	AzureCloudProvider     *AzureCloudProvider     `json:"azureCloudProvider,omitempty" yaml:"azureCloudProvider,omitempty"`
	CustomCloudProvider    string                  `json:"customCloudProvider,omitempty" yaml:"customCloudProvider,omitempty"`
	Name                   string                  `json:"name,omitempty" yaml:"name,omitempty"`
	OpenstackCloudProvider *OpenstackCloudProvider `json:"openstackCloudProvider,omitempty" yaml:"openstackCloudProvider,omitempty"`
	VsphereCloudProvider   *VsphereCloudProvider   `json:"vsphereCloudProvider,omitempty" yaml:"vsphereCloudProvider,omitempty"`
}
