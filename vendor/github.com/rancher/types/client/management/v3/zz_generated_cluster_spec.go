package client

const (
	ClusterSpecType                                    = "clusterSpec"
	ClusterSpecFieldAzureKubernetesServiceConfig       = "azureKubernetesServiceConfig"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateId = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldDescription                        = "description"
	ClusterSpecFieldGoogleKubernetesEngineConfig       = "googleKubernetesEngineConfig"
	ClusterSpecFieldInternal                           = "internal"
	ClusterSpecFieldRancherKubernetesEngineConfig      = "rancherKubernetesEngineConfig"
)

type ClusterSpec struct {
	AzureKubernetesServiceConfig       *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	DefaultPodSecurityPolicyTemplateId string                         `json:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                        string                         `json:"description,omitempty"`
	GoogleKubernetesEngineConfig       *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	Internal                           *bool                          `json:"internal,omitempty"`
	RancherKubernetesEngineConfig      *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
}
