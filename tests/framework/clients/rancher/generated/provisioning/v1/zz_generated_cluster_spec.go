package client

const (
	ClusterSpecType                                     = "clusterSpec"
	ClusterSpecFieldAgentEnvVars                        = "agentEnvVars"
	ClusterSpecFieldCloudCredentialSecretName           = "cloudCredentialSecretName"
	ClusterSpecFieldClusterAPIConfig                    = "clusterAPIConfig"
	ClusterSpecFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterSpecFieldDefaultPodSecurityPolicyTemplateID  = "defaultPodSecurityPolicyTemplateId"
	ClusterSpecFieldEnableNetworkPolicy                 = "enableNetworkPolicy"
	ClusterSpecFieldKubernetesVersion                   = "kubernetesVersion"
	ClusterSpecFieldLocalClusterAuthEndpoint            = "localClusterAuthEndpoint"
	ClusterSpecFieldRKEConfig                           = "rkeConfig"
	ClusterSpecFieldRedeploySystemAgentGeneration       = "redeploySystemAgentGeneration"
)

type ClusterSpec struct {
	AgentEnvVars                        []EnvVar                  `json:"agentEnvVars,omitempty" yaml:"agentEnvVars,omitempty"`
	CloudCredentialSecretName           string                    `json:"cloudCredentialSecretName,omitempty" yaml:"cloudCredentialSecretName,omitempty"`
	ClusterAPIConfig                    *ClusterAPIConfig         `json:"clusterAPIConfig,omitempty" yaml:"clusterAPIConfig,omitempty"`
	DefaultClusterRoleForProjectMembers string                    `json:"defaultClusterRoleForProjectMembers,omitempty" yaml:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateID  string                    `json:"defaultPodSecurityPolicyTemplateId,omitempty" yaml:"defaultPodSecurityPolicyTemplateId,omitempty"`
	EnableNetworkPolicy                 *bool                     `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	KubernetesVersion                   string                    `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LocalClusterAuthEndpoint            *LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty" yaml:"localClusterAuthEndpoint,omitempty"`
	RKEConfig                           *RKEConfig                `json:"rkeConfig,omitempty" yaml:"rkeConfig,omitempty"`
	RedeploySystemAgentGeneration       int64                     `json:"redeploySystemAgentGeneration,omitempty" yaml:"redeploySystemAgentGeneration,omitempty"`
}
