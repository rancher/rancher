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
	AzureKubernetesServiceConfigFieldSubscriptionID       = "subscriptionId"
	AzureKubernetesServiceConfigFieldTag                  = "tags"
	AzureKubernetesServiceConfigFieldTenantID             = "tenantId"
)

type AzureKubernetesServiceConfig struct {
	AdminUsername        string            `json:"adminUsername,omitempty"`
	AgentDNSPrefix       string            `json:"agentDnsPrefix,omitempty"`
	AgentPoolName        string            `json:"agentPoolName,omitempty"`
	AgentVMSize          string            `json:"agentVmSize,omitempty"`
	BaseURL              string            `json:"baseUrl,omitempty"`
	ClientID             string            `json:"clientId,omitempty"`
	ClientSecret         string            `json:"clientSecret,omitempty"`
	Count                *int64            `json:"count,omitempty"`
	KubernetesVersion    string            `json:"kubernetesVersion,omitempty"`
	Location             string            `json:"location,omitempty"`
	MasterDNSPrefix      string            `json:"masterDnsPrefix,omitempty"`
	OsDiskSizeGB         *int64            `json:"osDiskSizeGb,omitempty"`
	ResourceGroup        string            `json:"resourceGroup,omitempty"`
	SSHPublicKeyContents string            `json:"sshPublicKeyContents,omitempty"`
	SubscriptionID       string            `json:"subscriptionId,omitempty"`
	Tag                  map[string]string `json:"tags,omitempty"`
	TenantID             string            `json:"tenantId,omitempty"`
}
