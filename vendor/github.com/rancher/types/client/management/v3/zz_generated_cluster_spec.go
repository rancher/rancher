package client

const (
	ClusterSpecType                                     = "clusterSpec"
	ClusterSpecFieldAmazonElasticContainerServiceConfig = "amazonElasticContainerServiceConfig"
	ClusterSpecFieldAzureKubernetesServiceConfig        = "azureKubernetesServiceConfig"
	ClusterSpecFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateID  = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldDescription                         = "description"
	ClusterSpecFieldDesiredAgentImage                   = "desiredAgentImage"
	ClusterSpecFieldDisplayName                         = "displayName"
	ClusterSpecFieldGoogleKubernetesEngineConfig        = "googleKubernetesEngineConfig"
	ClusterSpecFieldImportedConfig                      = "importedConfig"
	ClusterSpecFieldInternal                            = "internal"
	ClusterSpecFieldRancherKubernetesEngineConfig       = "rancherKubernetesEngineConfig"
)

type ClusterSpec struct {
	AmazonElasticContainerServiceConfig *AmazonElasticContainerServiceConfig `json:"amazonElasticContainerServiceConfig,omitempty" yaml:"amazonElasticContainerServiceConfig,omitempty"`
	AzureKubernetesServiceConfig        *AzureKubernetesServiceConfig        `json:"azureKubernetesServiceConfig,omitempty" yaml:"azureKubernetesServiceConfig,omitempty"`
	DefaultClusterRoleForProjectMembers string                               `json:"defaultClusterRoleForProjectMembers,omitempty" yaml:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateID  string                               `json:"defaultPodSecurityPolicyTemplateId,omitempty" yaml:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                         string                               `json:"description,omitempty" yaml:"description,omitempty"`
	DesiredAgentImage                   string                               `json:"desiredAgentImage,omitempty" yaml:"desiredAgentImage,omitempty"`
	DisplayName                         string                               `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	GoogleKubernetesEngineConfig        *GoogleKubernetesEngineConfig        `json:"googleKubernetesEngineConfig,omitempty" yaml:"googleKubernetesEngineConfig,omitempty"`
	ImportedConfig                      *ImportedConfig                      `json:"importedConfig,omitempty" yaml:"importedConfig,omitempty"`
	Internal                            bool                                 `json:"internal,omitempty" yaml:"internal,omitempty"`
	RancherKubernetesEngineConfig       *RancherKubernetesEngineConfig       `json:"rancherKubernetesEngineConfig,omitempty" yaml:"rancherKubernetesEngineConfig,omitempty"`
}
