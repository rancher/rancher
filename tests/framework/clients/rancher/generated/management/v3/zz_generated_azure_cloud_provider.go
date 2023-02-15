package client

const (
	AzureCloudProviderType                              = "azureCloudProvider"
	AzureCloudProviderFieldAADClientCertPassword        = "aadClientCertPassword"
	AzureCloudProviderFieldAADClientCertPath            = "aadClientCertPath"
	AzureCloudProviderFieldAADClientID                  = "aadClientId"
	AzureCloudProviderFieldAADClientSecret              = "aadClientSecret"
	AzureCloudProviderFieldCloud                        = "cloud"
	AzureCloudProviderFieldCloudProviderBackoff         = "cloudProviderBackoff"
	AzureCloudProviderFieldCloudProviderBackoffDuration = "cloudProviderBackoffDuration"
	AzureCloudProviderFieldCloudProviderBackoffExponent = "cloudProviderBackoffExponent"
	AzureCloudProviderFieldCloudProviderBackoffJitter   = "cloudProviderBackoffJitter"
	AzureCloudProviderFieldCloudProviderBackoffRetries  = "cloudProviderBackoffRetries"
	AzureCloudProviderFieldCloudProviderRateLimit       = "cloudProviderRateLimit"
	AzureCloudProviderFieldCloudProviderRateLimitBucket = "cloudProviderRateLimitBucket"
	AzureCloudProviderFieldCloudProviderRateLimitQPS    = "cloudProviderRateLimitQPS"
	AzureCloudProviderFieldExcludeMasterFromStandardLB  = "excludeMasterFromStandardLB"
	AzureCloudProviderFieldLoadBalancerSku              = "loadBalancerSku"
	AzureCloudProviderFieldLocation                     = "location"
	AzureCloudProviderFieldMaximumLoadBalancerRuleCount = "maximumLoadBalancerRuleCount"
	AzureCloudProviderFieldPrimaryAvailabilitySetName   = "primaryAvailabilitySetName"
	AzureCloudProviderFieldPrimaryScaleSetName          = "primaryScaleSetName"
	AzureCloudProviderFieldResourceGroup                = "resourceGroup"
	AzureCloudProviderFieldRouteTableName               = "routeTableName"
	AzureCloudProviderFieldSecurityGroupName            = "securityGroupName"
	AzureCloudProviderFieldSecurityGroupResourceGroup   = "securityGroupResourceGroup"
	AzureCloudProviderFieldSubnetName                   = "subnetName"
	AzureCloudProviderFieldSubscriptionID               = "subscriptionId"
	AzureCloudProviderFieldTags                         = "tags"
	AzureCloudProviderFieldTenantID                     = "tenantId"
	AzureCloudProviderFieldUseInstanceMetadata          = "useInstanceMetadata"
	AzureCloudProviderFieldUseManagedIdentityExtension  = "useManagedIdentityExtension"
	AzureCloudProviderFieldUserAssignedIdentityID       = "userAssignedIdentityID"
	AzureCloudProviderFieldVMType                       = "vmType"
	AzureCloudProviderFieldVnetName                     = "vnetName"
	AzureCloudProviderFieldVnetResourceGroup            = "vnetResourceGroup"
)

type AzureCloudProvider struct {
	AADClientCertPassword        string `json:"aadClientCertPassword,omitempty" yaml:"aadClientCertPassword,omitempty"`
	AADClientCertPath            string `json:"aadClientCertPath,omitempty" yaml:"aadClientCertPath,omitempty"`
	AADClientID                  string `json:"aadClientId,omitempty" yaml:"aadClientId,omitempty"`
	AADClientSecret              string `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	Cloud                        string `json:"cloud,omitempty" yaml:"cloud,omitempty"`
	CloudProviderBackoff         bool   `json:"cloudProviderBackoff,omitempty" yaml:"cloudProviderBackoff,omitempty"`
	CloudProviderBackoffDuration int64  `json:"cloudProviderBackoffDuration,omitempty" yaml:"cloudProviderBackoffDuration,omitempty"`
	CloudProviderBackoffExponent int64  `json:"cloudProviderBackoffExponent,omitempty" yaml:"cloudProviderBackoffExponent,omitempty"`
	CloudProviderBackoffJitter   int64  `json:"cloudProviderBackoffJitter,omitempty" yaml:"cloudProviderBackoffJitter,omitempty"`
	CloudProviderBackoffRetries  int64  `json:"cloudProviderBackoffRetries,omitempty" yaml:"cloudProviderBackoffRetries,omitempty"`
	CloudProviderRateLimit       bool   `json:"cloudProviderRateLimit,omitempty" yaml:"cloudProviderRateLimit,omitempty"`
	CloudProviderRateLimitBucket int64  `json:"cloudProviderRateLimitBucket,omitempty" yaml:"cloudProviderRateLimitBucket,omitempty"`
	CloudProviderRateLimitQPS    int64  `json:"cloudProviderRateLimitQPS,omitempty" yaml:"cloudProviderRateLimitQPS,omitempty"`
	ExcludeMasterFromStandardLB  *bool  `json:"excludeMasterFromStandardLB,omitempty" yaml:"excludeMasterFromStandardLB,omitempty"`
	LoadBalancerSku              string `json:"loadBalancerSku,omitempty" yaml:"loadBalancerSku,omitempty"`
	Location                     string `json:"location,omitempty" yaml:"location,omitempty"`
	MaximumLoadBalancerRuleCount int64  `json:"maximumLoadBalancerRuleCount,omitempty" yaml:"maximumLoadBalancerRuleCount,omitempty"`
	PrimaryAvailabilitySetName   string `json:"primaryAvailabilitySetName,omitempty" yaml:"primaryAvailabilitySetName,omitempty"`
	PrimaryScaleSetName          string `json:"primaryScaleSetName,omitempty" yaml:"primaryScaleSetName,omitempty"`
	ResourceGroup                string `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	RouteTableName               string `json:"routeTableName,omitempty" yaml:"routeTableName,omitempty"`
	SecurityGroupName            string `json:"securityGroupName,omitempty" yaml:"securityGroupName,omitempty"`
	SecurityGroupResourceGroup   string `json:"securityGroupResourceGroup,omitempty" yaml:"securityGroupResourceGroup,omitempty"`
	SubnetName                   string `json:"subnetName,omitempty" yaml:"subnetName,omitempty"`
	SubscriptionID               string `json:"subscriptionId,omitempty" yaml:"subscriptionId,omitempty"`
	Tags                         string `json:"tags,omitempty" yaml:"tags,omitempty"`
	TenantID                     string `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	UseInstanceMetadata          bool   `json:"useInstanceMetadata,omitempty" yaml:"useInstanceMetadata,omitempty"`
	UseManagedIdentityExtension  bool   `json:"useManagedIdentityExtension,omitempty" yaml:"useManagedIdentityExtension,omitempty"`
	UserAssignedIdentityID       string `json:"userAssignedIdentityID,omitempty" yaml:"userAssignedIdentityID,omitempty"`
	VMType                       string `json:"vmType,omitempty" yaml:"vmType,omitempty"`
	VnetName                     string `json:"vnetName,omitempty" yaml:"vnetName,omitempty"`
	VnetResourceGroup            string `json:"vnetResourceGroup,omitempty" yaml:"vnetResourceGroup,omitempty"`
}
