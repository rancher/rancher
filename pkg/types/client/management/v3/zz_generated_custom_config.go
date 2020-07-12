package client

const (
	CustomConfigType                 = "customConfig"
	CustomConfigFieldAddress         = "address"
	CustomConfigFieldDockerSocket    = "dockerSocket"
	CustomConfigFieldInternalAddress = "internalAddress"
	CustomConfigFieldLabel           = "label"
	CustomConfigFieldSSHCert         = "sshCert"
	CustomConfigFieldSSHKey          = "sshKey"
	CustomConfigFieldTaints          = "taints"
	CustomConfigFieldUser            = "user"
)

type CustomConfig struct {
	Address         string            `json:"address,omitempty" yaml:"address,omitempty"`
	DockerSocket    string            `json:"dockerSocket,omitempty" yaml:"dockerSocket,omitempty"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Label           map[string]string `json:"label,omitempty" yaml:"label,omitempty"`
	SSHCert         string            `json:"sshCert,omitempty" yaml:"sshCert,omitempty"`
	SSHKey          string            `json:"sshKey,omitempty" yaml:"sshKey,omitempty"`
	Taints          []string          `json:"taints,omitempty" yaml:"taints,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
}
