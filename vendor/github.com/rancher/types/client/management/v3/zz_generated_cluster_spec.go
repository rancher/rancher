package client

const (
	ClusterSpecType                                     = "clusterSpec"
	ClusterSpecFieldAzureKubernetesServiceConfig        = "azureKubernetesServiceConfig"
	ClusterSpecFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateId  = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldDescription                         = "description"
	ClusterSpecFieldDisplayName                         = "displayName"
	ClusterSpecFieldEmbeddedConfig                      = "embeddedConfig"
	ClusterSpecFieldGoogleKubernetesEngineConfig        = "googleKubernetesEngineConfig"
	ClusterSpecFieldImportedConfig                      = "importedConfig"
	ClusterSpecFieldInternal                            = "internal"
	ClusterSpecFieldNodes                               = "nodes"
	ClusterSpecFieldRancherKubernetesEngineConfig       = "rancherKubernetesEngineConfig"
)

type ClusterSpec struct {
	AzureKubernetesServiceConfig        *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	DefaultClusterRoleForProjectMembers string                         `json:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateId  string                         `json:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                         string                         `json:"description,omitempty"`
	DisplayName                         string                         `json:"displayName,omitempty"`
	EmbeddedConfig                      *K8sServerConfig               `json:"embeddedConfig,omitempty"`
	GoogleKubernetesEngineConfig        *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	ImportedConfig                      *ImportedConfig                `json:"importedConfig,omitempty"`
	Internal                            *bool                          `json:"internal,omitempty"`
	Nodes                               []MachineConfig                `json:"nodes,omitempty"`
	RancherKubernetesEngineConfig       *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
}
