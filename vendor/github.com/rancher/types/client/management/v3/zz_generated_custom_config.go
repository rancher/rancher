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
	Address         string `json:"address,omitempty"`
	DockerSocket    string `json:"dockerSocket,omitempty"`
	InternalAddress string `json:"internalAddress,omitempty"`
	SSHKey          string `json:"sshKey,omitempty"`
	User            string `json:"user,omitempty"`
}
