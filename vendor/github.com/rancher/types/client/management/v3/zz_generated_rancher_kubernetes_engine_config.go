package client

const (
	RancherKubernetesEngineConfigType                     = "rancherKubernetesEngineConfig"
	RancherKubernetesEngineConfigFieldAddons              = "addons"
	RancherKubernetesEngineConfigFieldAuthentication      = "authentication"
	RancherKubernetesEngineConfigFieldAuthorization       = "authorization"
	RancherKubernetesEngineConfigFieldDisablePSP          = "disablePSP"
	RancherKubernetesEngineConfigFieldIgnoreDockerVersion = "ignoreDockerVersion"
	RancherKubernetesEngineConfigFieldNetwork             = "network"
	RancherKubernetesEngineConfigFieldNodes               = "nodes"
	RancherKubernetesEngineConfigFieldSSHKeyPath          = "sshKeyPath"
	RancherKubernetesEngineConfigFieldServices            = "services"
	RancherKubernetesEngineConfigFieldSystemImages        = "systemImages"
)

type RancherKubernetesEngineConfig struct {
	Addons              string             `json:"addons,omitempty"`
	Authentication      *AuthnConfig       `json:"authentication,omitempty"`
	Authorization       *AuthzConfig       `json:"authorization,omitempty"`
	DisablePSP          *bool              `json:"disablePSP,omitempty"`
	IgnoreDockerVersion *bool              `json:"ignoreDockerVersion,omitempty"`
	Network             *NetworkConfig     `json:"network,omitempty"`
	Nodes               []RKEConfigNode    `json:"nodes,omitempty"`
	SSHKeyPath          string             `json:"sshKeyPath,omitempty"`
	Services            *RKEConfigServices `json:"services,omitempty"`
	SystemImages        map[string]string  `json:"systemImages,omitempty"`
}
