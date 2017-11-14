package client

const (
	ClusterSpecType                               = "clusterSpec"
	ClusterSpecFieldAzureKubernetesServiceConfig  = "azureKubernetesServiceConfig"
	ClusterSpecFieldDescription                   = "description"
	ClusterSpecFieldGoogleKubernetesEngineConfig  = "googleKubernetesEngineConfig"
	ClusterSpecFieldInternal                      = "internal"
	ClusterSpecFieldRancherKubernetesEngineConfig = "rancherKubernetesEngineConfig"
)

type ClusterSpec struct {
	AzureKubernetesServiceConfig  *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	Description                   string                         `json:"description,omitempty"`
	GoogleKubernetesEngineConfig  *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	Internal                      *bool                          `json:"internal,omitempty"`
	RancherKubernetesEngineConfig *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
}
