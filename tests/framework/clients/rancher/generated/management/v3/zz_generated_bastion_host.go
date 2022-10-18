package client

const (
	BastionHostType                    = "bastionHost"
	BastionHostFieldAddress            = "address"
	BastionHostFieldIgnoreProxyEnvVars = "ignoreProxyEnvVars"
	BastionHostFieldPort               = "port"
	BastionHostFieldSSHAgentAuth       = "sshAgentAuth"
	BastionHostFieldSSHCert            = "sshCert"
	BastionHostFieldSSHCertPath        = "sshCertPath"
	BastionHostFieldSSHKey             = "sshKey"
	BastionHostFieldSSHKeyPath         = "sshKeyPath"
	BastionHostFieldUser               = "user"
)

type BastionHost struct {
	Address            string `json:"address,omitempty" yaml:"address,omitempty"`
	IgnoreProxyEnvVars bool   `json:"ignoreProxyEnvVars,omitempty" yaml:"ignoreProxyEnvVars,omitempty"`
	Port               string `json:"port,omitempty" yaml:"port,omitempty"`
	SSHAgentAuth       bool   `json:"sshAgentAuth,omitempty" yaml:"sshAgentAuth,omitempty"`
	SSHCert            string `json:"sshCert,omitempty" yaml:"sshCert,omitempty"`
	SSHCertPath        string `json:"sshCertPath,omitempty" yaml:"sshCertPath,omitempty"`
	SSHKey             string `json:"sshKey,omitempty" yaml:"sshKey,omitempty"`
	SSHKeyPath         string `json:"sshKeyPath,omitempty" yaml:"sshKeyPath,omitempty"`
	User               string `json:"user,omitempty" yaml:"user,omitempty"`
}
