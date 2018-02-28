package client

const (
	CustomConfigType                 = "customConfig"
	CustomConfigFieldAddress         = "address"
	CustomConfigFieldDockerSocket    = "dockerSocket"
	CustomConfigFieldInternalAddress = "internalAddress"
	CustomConfigFieldSSHKey          = "sshKey"
	CustomConfigFieldUser            = "user"
)

type CustomConfig struct {
	Address         string `json:"address,omitempty" yaml:"address,omitempty"`
	DockerSocket    string `json:"dockerSocket,omitempty" yaml:"dockerSocket,omitempty"`
	InternalAddress string `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	SSHKey          string `json:"sshKey,omitempty" yaml:"sshKey,omitempty"`
	User            string `json:"user,omitempty" yaml:"user,omitempty"`
}
