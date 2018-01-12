package client

const (
	ClusterSpecType                                     = "clusterSpec"
	ClusterSpecFieldAzureKubernetesServiceConfig        = "azureKubernetesServiceConfig"
	ClusterSpecFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateId  = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldDescription                         = "description"
	ClusterSpecFieldGoogleKubernetesEngineConfig        = "googleKubernetesEngineConfig"
	ClusterSpecFieldInternal                            = "internal"
	ClusterSpecFieldNodes                               = "nodes"
	ClusterSpecFieldRancherKubernetesEngineConfig       = "rancherKubernetesEngineConfig"
)

type ClusterSpec struct {
	AzureKubernetesServiceConfig        *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	DefaultClusterRoleForProjectMembers string                         `json:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateId  string                         `json:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                         string                         `json:"description,omitempty"`
	GoogleKubernetesEngineConfig        *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	Internal                            *bool                          `json:"internal,omitempty"`
	Nodes                               []MachineConfig                `json:"nodes,omitempty"`
	RancherKubernetesEngineConfig       *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
}
