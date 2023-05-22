package client

const (
	CloudProviderType                             = "cloudProvider"
	CloudProviderFieldAWSCloudProvider            = "awsCloudProvider"
	CloudProviderFieldAzureCloudProvider          = "azureCloudProvider"
	CloudProviderFieldCustomCloudProvider         = "customCloudProvider"
	CloudProviderFieldHarvesterCloudProvider      = "harvesterCloudProvider"
	CloudProviderFieldName                        = "name"
	CloudProviderFieldOpenstackCloudProvider      = "openstackCloudProvider"
	CloudProviderFieldUseInstanceMetadataHostname = "useInstanceMetadataHostname"
	CloudProviderFieldVsphereCloudProvider        = "vsphereCloudProvider"
)

type CloudProvider struct {
	AWSCloudProvider            *AWSCloudProvider       `json:"awsCloudProvider,omitempty" yaml:"awsCloudProvider,omitempty"`
	AzureCloudProvider          *AzureCloudProvider     `json:"azureCloudProvider,omitempty" yaml:"azureCloudProvider,omitempty"`
	CustomCloudProvider         string                  `json:"customCloudProvider,omitempty" yaml:"customCloudProvider,omitempty"`
	HarvesterCloudProvider      *HarvesterCloudProvider `json:"harvesterCloudProvider,omitempty" yaml:"harvesterCloudProvider,omitempty"`
	Name                        string                  `json:"name,omitempty" yaml:"name,omitempty"`
	OpenstackCloudProvider      *OpenstackCloudProvider `json:"openstackCloudProvider,omitempty" yaml:"openstackCloudProvider,omitempty"`
	UseInstanceMetadataHostname *bool                   `json:"useInstanceMetadataHostname,omitempty" yaml:"useInstanceMetadataHostname,omitempty"`
	VsphereCloudProvider        *VsphereCloudProvider   `json:"vsphereCloudProvider,omitempty" yaml:"vsphereCloudProvider,omitempty"`
}
