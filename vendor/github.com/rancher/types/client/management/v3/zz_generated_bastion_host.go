package client

const (
	BastionHostType              = "bastionHost"
	BastionHostFieldAddress      = "address"
	BastionHostFieldPort         = "port"
	BastionHostFieldSSHAgentAuth = "sshAgentAuth"
	BastionHostFieldSSHKey       = "sshKey"
	BastionHostFieldSSHKeyPath   = "sshKeyPath"
	BastionHostFieldUser         = "user"
)

type BastionHost struct {
	Address      string `json:"address,omitempty" yaml:"address,omitempty"`
	Port         string `json:"port,omitempty" yaml:"port,omitempty"`
	SSHAgentAuth bool   `json:"sshAgentAuth,omitempty" yaml:"sshAgentAuth,omitempty"`
	SSHKey       string `json:"sshKey,omitempty" yaml:"sshKey,omitempty"`
	SSHKeyPath   string `json:"sshKeyPath,omitempty" yaml:"sshKeyPath,omitempty"`
	User         string `json:"user,omitempty" yaml:"user,omitempty"`
}
