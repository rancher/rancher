package client

const (
	RancherKubernetesEngineConfigType                     = "rancherKubernetesEngineConfig"
	RancherKubernetesEngineConfigFieldAddons              = "addons"
	RancherKubernetesEngineConfigFieldAddonsInclude       = "addonsInclude"
	RancherKubernetesEngineConfigFieldAuthentication      = "authentication"
	RancherKubernetesEngineConfigFieldAuthorization       = "authorization"
	RancherKubernetesEngineConfigFieldCloudProvider       = "cloudProvider"
	RancherKubernetesEngineConfigFieldClusterName         = "clusterName"
	RancherKubernetesEngineConfigFieldIgnoreDockerVersion = "ignoreDockerVersion"
	RancherKubernetesEngineConfigFieldIngress             = "ingress"
	RancherKubernetesEngineConfigFieldNetwork             = "network"
	RancherKubernetesEngineConfigFieldNodes               = "nodes"
	RancherKubernetesEngineConfigFieldPrefixPath          = "prefixPath"
	RancherKubernetesEngineConfigFieldPrivateRegistries   = "privateRegistries"
	RancherKubernetesEngineConfigFieldSSHAgentAuth        = "sshAgentAuth"
	RancherKubernetesEngineConfigFieldSSHKeyPath          = "sshKeyPath"
	RancherKubernetesEngineConfigFieldServices            = "services"
	RancherKubernetesEngineConfigFieldVersion             = "kubernetesVersion"
)

type RancherKubernetesEngineConfig struct {
	Addons              string             `json:"addons,omitempty" yaml:"addons,omitempty"`
	AddonsInclude       []string           `json:"addonsInclude,omitempty" yaml:"addonsInclude,omitempty"`
	Authentication      *AuthnConfig       `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	Authorization       *AuthzConfig       `json:"authorization,omitempty" yaml:"authorization,omitempty"`
	CloudProvider       *CloudProvider     `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`
	ClusterName         string             `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	IgnoreDockerVersion bool               `json:"ignoreDockerVersion,omitempty" yaml:"ignoreDockerVersion,omitempty"`
	Ingress             *IngressConfig     `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	Network             *NetworkConfig     `json:"network,omitempty" yaml:"network,omitempty"`
	Nodes               []RKEConfigNode    `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	PrefixPath          string             `json:"prefixPath,omitempty" yaml:"prefixPath,omitempty"`
	PrivateRegistries   []PrivateRegistry  `json:"privateRegistries,omitempty" yaml:"privateRegistries,omitempty"`
	SSHAgentAuth        bool               `json:"sshAgentAuth,omitempty" yaml:"sshAgentAuth,omitempty"`
	SSHKeyPath          string             `json:"sshKeyPath,omitempty" yaml:"sshKeyPath,omitempty"`
	Services            *RKEConfigServices `json:"services,omitempty" yaml:"services,omitempty"`
	Version             string             `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}
