package client

const (
	ClusterSpecType                                     = "clusterSpec"
	ClusterSpecFieldAgentImageOverride                  = "agentImageOverride"
	ClusterSpecFieldAmazonElasticContainerServiceConfig = "amazonElasticContainerServiceConfig"
	ClusterSpecFieldAzureKubernetesServiceConfig        = "azureKubernetesServiceConfig"
	ClusterSpecFieldClusterTemplateAnswers              = "answers"
	ClusterSpecFieldClusterTemplateID                   = "clusterTemplateId"
	ClusterSpecFieldClusterTemplateQuestions            = "questions"
	ClusterSpecFieldClusterTemplateRevisionID           = "clusterTemplateRevisionId"
	ClusterSpecFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateID  = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldDescription                         = "description"
	ClusterSpecFieldDesiredAgentImage                   = "desiredAgentImage"
	ClusterSpecFieldDesiredAuthImage                    = "desiredAuthImage"
	ClusterSpecFieldDisplayName                         = "displayName"
	ClusterSpecFieldDockerRootDir                       = "dockerRootDir"
	ClusterSpecFieldEKSConfig                           = "eksConfig"
	ClusterSpecFieldEnableClusterAlerting               = "enableClusterAlerting"
	ClusterSpecFieldEnableClusterMonitoring             = "enableClusterMonitoring"
	ClusterSpecFieldEnableNetworkPolicy                 = "enableNetworkPolicy"
	ClusterSpecFieldFleetWorkspaceName                  = "fleetWorkspaceName"
	ClusterSpecFieldGenericEngineConfig                 = "genericEngineConfig"
	ClusterSpecFieldGoogleKubernetesEngineConfig        = "googleKubernetesEngineConfig"
	ClusterSpecFieldImportedConfig                      = "importedConfig"
	ClusterSpecFieldInternal                            = "internal"
	ClusterSpecFieldK3sConfig                           = "k3sConfig"
	ClusterSpecFieldLocalClusterAuthEndpoint            = "localClusterAuthEndpoint"
	ClusterSpecFieldRancherKubernetesEngineConfig       = "rancherKubernetesEngineConfig"
	ClusterSpecFieldRke2Config                          = "rke2Config"
	ClusterSpecFieldScheduledClusterScan                = "scheduledClusterScan"
	ClusterSpecFieldWindowsPreferedCluster              = "windowsPreferedCluster"
)

type ClusterSpec struct {
	AgentImageOverride                  string                         `json:"agentImageOverride,omitempty" yaml:"agentImageOverride,omitempty"`
	AmazonElasticContainerServiceConfig map[string]interface{}         `json:"amazonElasticContainerServiceConfig,omitempty" yaml:"amazonElasticContainerServiceConfig,omitempty"`
	AzureKubernetesServiceConfig        map[string]interface{}         `json:"azureKubernetesServiceConfig,omitempty" yaml:"azureKubernetesServiceConfig,omitempty"`
	ClusterTemplateAnswers              *Answer                        `json:"answers,omitempty" yaml:"answers,omitempty"`
	ClusterTemplateID                   string                         `json:"clusterTemplateId,omitempty" yaml:"clusterTemplateId,omitempty"`
	ClusterTemplateQuestions            []Question                     `json:"questions,omitempty" yaml:"questions,omitempty"`
	ClusterTemplateRevisionID           string                         `json:"clusterTemplateRevisionId,omitempty" yaml:"clusterTemplateRevisionId,omitempty"`
	DefaultClusterRoleForProjectMembers string                         `json:"defaultClusterRoleForProjectMembers,omitempty" yaml:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateID  string                         `json:"defaultPodSecurityPolicyTemplateId,omitempty" yaml:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                         string                         `json:"description,omitempty" yaml:"description,omitempty"`
	DesiredAgentImage                   string                         `json:"desiredAgentImage,omitempty" yaml:"desiredAgentImage,omitempty"`
	DesiredAuthImage                    string                         `json:"desiredAuthImage,omitempty" yaml:"desiredAuthImage,omitempty"`
	DisplayName                         string                         `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DockerRootDir                       string                         `json:"dockerRootDir,omitempty" yaml:"dockerRootDir,omitempty"`
	EKSConfig                           *EKSClusterConfigSpec          `json:"eksConfig,omitempty" yaml:"eksConfig,omitempty"`
	EnableClusterAlerting               bool                           `json:"enableClusterAlerting,omitempty" yaml:"enableClusterAlerting,omitempty"`
	EnableClusterMonitoring             bool                           `json:"enableClusterMonitoring,omitempty" yaml:"enableClusterMonitoring,omitempty"`
	EnableNetworkPolicy                 *bool                          `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	FleetWorkspaceName                  string                         `json:"fleetWorkspaceName,omitempty" yaml:"fleetWorkspaceName,omitempty"`
	GenericEngineConfig                 map[string]interface{}         `json:"genericEngineConfig,omitempty" yaml:"genericEngineConfig,omitempty"`
	GoogleKubernetesEngineConfig        map[string]interface{}         `json:"googleKubernetesEngineConfig,omitempty" yaml:"googleKubernetesEngineConfig,omitempty"`
	ImportedConfig                      *ImportedConfig                `json:"importedConfig,omitempty" yaml:"importedConfig,omitempty"`
	Internal                            bool                           `json:"internal,omitempty" yaml:"internal,omitempty"`
	K3sConfig                           *K3sConfig                     `json:"k3sConfig,omitempty" yaml:"k3sConfig,omitempty"`
	LocalClusterAuthEndpoint            *LocalClusterAuthEndpoint      `json:"localClusterAuthEndpoint,omitempty" yaml:"localClusterAuthEndpoint,omitempty"`
	RancherKubernetesEngineConfig       *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty" yaml:"rancherKubernetesEngineConfig,omitempty"`
	Rke2Config                          *Rke2Config                    `json:"rke2Config,omitempty" yaml:"rke2Config,omitempty"`
	ScheduledClusterScan                *ScheduledClusterScan          `json:"scheduledClusterScan,omitempty" yaml:"scheduledClusterScan,omitempty"`
	WindowsPreferedCluster              bool                           `json:"windowsPreferedCluster,omitempty" yaml:"windowsPreferedCluster,omitempty"`
}
