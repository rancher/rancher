package client

const (
	RancherKubernetesEngineConfigType                     = "rancherKubernetesEngineConfig"
	RancherKubernetesEngineConfigFieldAddons              = "addons"
	RancherKubernetesEngineConfigFieldAuthentication      = "authentication"
	RancherKubernetesEngineConfigFieldAuthorization       = "authorization"
	RancherKubernetesEngineConfigFieldIgnoreDockerVersion = "ignoreDockerVersion"
	RancherKubernetesEngineConfigFieldNetwork             = "network"
	RancherKubernetesEngineConfigFieldNodes               = "nodes"
	RancherKubernetesEngineConfigFieldPrivateRegistries   = "privateRegistries"
	RancherKubernetesEngineConfigFieldSSHKeyPath          = "sshKeyPath"
	RancherKubernetesEngineConfigFieldServices            = "services"
	RancherKubernetesEngineConfigFieldSystemImages        = "systemImages"
	RancherKubernetesEngineConfigFieldVersion             = "kubernetesVersion"
)

type RancherKubernetesEngineConfig struct {
	Addons              string             `json:"addons,omitempty"`
	Authentication      *AuthnConfig       `json:"authentication,omitempty"`
	Authorization       *AuthzConfig       `json:"authorization,omitempty"`
	IgnoreDockerVersion *bool              `json:"ignoreDockerVersion,omitempty"`
	Network             *NetworkConfig     `json:"network,omitempty"`
	Nodes               []RKEConfigNode    `json:"nodes,omitempty"`
	PrivateRegistries   []PrivateRegistry  `json:"privateRegistries,omitempty"`
	SSHKeyPath          string             `json:"sshKeyPath,omitempty"`
	Services            *RKEConfigServices `json:"services,omitempty"`
	SystemImages        *RKESystemImages   `json:"systemImages,omitempty"`
	Version             string             `json:"kubernetesVersion,omitempty"`
}
