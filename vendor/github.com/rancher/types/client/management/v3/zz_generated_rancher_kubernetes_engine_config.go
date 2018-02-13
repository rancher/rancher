package client

const (
	RancherKubernetesEngineConfigType                     = "rancherKubernetesEngineConfig"
	RancherKubernetesEngineConfigFieldAddons              = "addons"
	RancherKubernetesEngineConfigFieldAuthentication      = "authentication"
	RancherKubernetesEngineConfigFieldAuthorization       = "authorization"
	RancherKubernetesEngineConfigFieldIgnoreDockerVersion = "ignoreDockerVersion"
	RancherKubernetesEngineConfigFieldIngress             = "ingress"
	RancherKubernetesEngineConfigFieldNetwork             = "network"
	RancherKubernetesEngineConfigFieldNodes               = "nodes"
	RancherKubernetesEngineConfigFieldPrivateRegistries   = "privateRegistries"
	RancherKubernetesEngineConfigFieldSSHKeyPath          = "sshKeyPath"
	RancherKubernetesEngineConfigFieldServices            = "services"
	RancherKubernetesEngineConfigFieldVersion             = "kubernetesVersion"
)

type RancherKubernetesEngineConfig struct {
	Addons              string             `json:"addons,omitempty"`
	Authentication      *AuthnConfig       `json:"authentication,omitempty"`
	Authorization       *AuthzConfig       `json:"authorization,omitempty"`
	IgnoreDockerVersion bool               `json:"ignoreDockerVersion,omitempty"`
	Ingress             *IngressConfig     `json:"ingress,omitempty"`
	Network             *NetworkConfig     `json:"network,omitempty"`
	Nodes               []RKEConfigNode    `json:"nodes,omitempty"`
	PrivateRegistries   []PrivateRegistry  `json:"privateRegistries,omitempty"`
	SSHKeyPath          string             `json:"sshKeyPath,omitempty"`
	Services            *RKEConfigServices `json:"services,omitempty"`
	Version             string             `json:"kubernetesVersion,omitempty"`
}
