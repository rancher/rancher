package client

const (
	AzureKubernetesServiceConfigType                      = "azureKubernetesServiceConfig"
	AzureKubernetesServiceConfigFieldAdminUsername        = "adminUsername"
	AzureKubernetesServiceConfigFieldAgentDNSPrefix       = "agentDnsPrefix"
	AzureKubernetesServiceConfigFieldAgentPoolName        = "agentPoolName"
	AzureKubernetesServiceConfigFieldAgentVMSize          = "agentVmSize"
	AzureKubernetesServiceConfigFieldBaseURL              = "baseUrl"
	AzureKubernetesServiceConfigFieldClientID             = "clientId"
	AzureKubernetesServiceConfigFieldClientSecret         = "clientSecret"
	AzureKubernetesServiceConfigFieldCount                = "count"
	AzureKubernetesServiceConfigFieldKubernetesVersion    = "kubernetesVersion"
	AzureKubernetesServiceConfigFieldLocation             = "location"
	AzureKubernetesServiceConfigFieldMasterDNSPrefix      = "masterDnsPrefix"
	AzureKubernetesServiceConfigFieldOsDiskSizeGB         = "osDiskSizeGb"
	AzureKubernetesServiceConfigFieldResourceGroup        = "resourceGroup"
	AzureKubernetesServiceConfigFieldSSHPublicKeyContents = "sshPublicKeyContents"
	AzureKubernetesServiceConfigFieldSubnet               = "subnet"
	AzureKubernetesServiceConfigFieldSubscriptionID       = "subscriptionId"
	AzureKubernetesServiceConfigFieldTag                  = "tags"
	AzureKubernetesServiceConfigFieldTenantID             = "tenantId"
	AzureKubernetesServiceConfigFieldVirtualNetwork       = "virtualNetwork"
)

type AzureKubernetesServiceConfig struct {
	AdminUsername        string            `json:"adminUsername,omitempty" yaml:"adminUsername,omitempty"`
	AgentDNSPrefix       string            `json:"agentDnsPrefix,omitempty" yaml:"agentDnsPrefix,omitempty"`
	AgentPoolName        string            `json:"agentPoolName,omitempty" yaml:"agentPoolName,omitempty"`
	AgentVMSize          string            `json:"agentVmSize,omitempty" yaml:"agentVmSize,omitempty"`
	BaseURL              string            `json:"baseUrl,omitempty" yaml:"baseUrl,omitempty"`
	ClientID             string            `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret         string            `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	Count                int64             `json:"count,omitempty" yaml:"count,omitempty"`
	KubernetesVersion    string            `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	Location             string            `json:"location,omitempty" yaml:"location,omitempty"`
	MasterDNSPrefix      string            `json:"masterDnsPrefix,omitempty" yaml:"masterDnsPrefix,omitempty"`
	OsDiskSizeGB         int64             `json:"osDiskSizeGb,omitempty" yaml:"osDiskSizeGb,omitempty"`
	ResourceGroup        string            `json:"resourceGroup,omitempty" yaml:"resourceGroup,omitempty"`
	SSHPublicKeyContents string            `json:"sshPublicKeyContents,omitempty" yaml:"sshPublicKeyContents,omitempty"`
	Subnet               string            `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	SubscriptionID       string            `json:"subscriptionId,omitempty" yaml:"subscriptionId,omitempty"`
	Tag                  map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	TenantID             string            `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	VirtualNetwork       string            `json:"virtualNetwork,omitempty" yaml:"virtualNetwork,omitempty"`
}
