package client

const (
	RancherKubernetesEngineConfigType                        = "rancherKubernetesEngineConfig"
	RancherKubernetesEngineConfigFieldAWSCloudProvider       = "awsCloudProvider"
	RancherKubernetesEngineConfigFieldAddons                 = "addons"
	RancherKubernetesEngineConfigFieldAddonsInclude          = "addonsInclude"
	RancherKubernetesEngineConfigFieldAuthentication         = "authentication"
	RancherKubernetesEngineConfigFieldAuthorization          = "authorization"
	RancherKubernetesEngineConfigFieldAzureCloudProvider     = "azureCloudProvider"
	RancherKubernetesEngineConfigFieldCalicoNetworkProvider  = "calicoNetworkProvider"
	RancherKubernetesEngineConfigFieldCanalNetworkProvider   = "canalNetworkProvider"
	RancherKubernetesEngineConfigFieldCloudProvider          = "cloudProvider"
	RancherKubernetesEngineConfigFieldClusterName            = "clusterName"
	RancherKubernetesEngineConfigFieldFlannelNetworkProvider = "flannelNetworkProvider"
	RancherKubernetesEngineConfigFieldIgnoreDockerVersion    = "ignoreDockerVersion"
	RancherKubernetesEngineConfigFieldIngress                = "ingress"
	RancherKubernetesEngineConfigFieldNetwork                = "network"
	RancherKubernetesEngineConfigFieldNodes                  = "nodes"
	RancherKubernetesEngineConfigFieldPrivateRegistries      = "privateRegistries"
	RancherKubernetesEngineConfigFieldSSHAgentAuth           = "sshAgentAuth"
	RancherKubernetesEngineConfigFieldSSHKeyPath             = "sshKeyPath"
	RancherKubernetesEngineConfigFieldServices               = "services"
	RancherKubernetesEngineConfigFieldVersion                = "kubernetesVersion"
)

type RancherKubernetesEngineConfig struct {
	AWSCloudProvider       *AWSCloudProvider       `json:"awsCloudProvider,omitempty" yaml:"awsCloudProvider,omitempty"`
	Addons                 string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	AddonsInclude          []string                `json:"addonsInclude,omitempty" yaml:"addonsInclude,omitempty"`
	Authentication         *AuthnConfig            `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	Authorization          *AuthzConfig            `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	AzureCloudProvider     *AzureCloudProvider     `json:"azureCloudProvider,omitempty" yaml:"azureCloudProvider,omitempty"`
	CalicoNetworkProvider  *CalicoNetworkProvider  `json:"calicoNetworkProvider,omitempty" yaml:"calicoNetworkProvider,omitempty"`
	CanalNetworkProvider   *CanalNetworkProvider   `json:"canalNetworkProvider,omitempty" yaml:"canalNetworkProvider,omitempty"`
	CloudProvider          *CloudProvider          `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`
	ClusterName            string                  `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	FlannelNetworkProvider *FlannelNetworkProvider `json:"flannelNetworkProvider,omitempty" yaml:"flannelNetworkProvider,omitempty"`
	IgnoreDockerVersion    bool                    `json:"ignoreDockerVersion,omitempty" yaml:"ignoreDockerVersion,omitempty"`
	Ingress                *IngressConfig          `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	Network                *NetworkConfig          `json:"network,omitempty" yaml:"network,omitempty"`
	Nodes                  []RKEConfigNode         `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	PrivateRegistries      []PrivateRegistry       `json:"privateRegistries,omitempty" yaml:"privateRegistries,omitempty"`
	SSHAgentAuth           bool                    `json:"sshAgentAuth,omitempty" yaml:"sshAgentAuth,omitempty"`
	SSHKeyPath             string                  `json:"sshKeyPath,omitempty" yaml:"sshKeyPath,omitempty"`
	Services               *RKEConfigServices      `json:"services,omitempty" yaml:"services,omitempty"`
	Version                string                  `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
