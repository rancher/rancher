package client

const (
	RKEConfigNodeType                  = "rkeConfigNode"
	RKEConfigNodeFieldAddress          = "address"
	RKEConfigNodeFieldDockerSocket     = "dockerSocket"
	RKEConfigNodeFieldHostnameOverride = "hostnameOverride"
	RKEConfigNodeFieldInternalAddress  = "internalAddress"
	RKEConfigNodeFieldLabels           = "labels"
	RKEConfigNodeFieldNodeID           = "nodeId"
	RKEConfigNodeFieldPort             = "port"
	RKEConfigNodeFieldRole             = "role"
	RKEConfigNodeFieldSSHAgentAuth     = "sshAgentAuth"
	RKEConfigNodeFieldSSHCert          = "sshCert"
	RKEConfigNodeFieldSSHCertPath      = "sshCertPath"
	RKEConfigNodeFieldSSHKey           = "sshKey"
	RKEConfigNodeFieldSSHKeyPath       = "sshKeyPath"
	RKEConfigNodeFieldTaints           = "taints"
	RKEConfigNodeFieldUser             = "user"
)

type RKEConfigNode struct {
	Address          string            `json:"address,omitempty" yaml:"address,omitempty"`
	DockerSocket     string            `json:"dockerSocket,omitempty" yaml:"dockerSocket,omitempty"`
	HostnameOverride string            `json:"hostnameOverride,omitempty" yaml:"hostnameOverride,omitempty"`
	InternalAddress  string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	NodeID           string            `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Port             string            `json:"port,omitempty" yaml:"port,omitempty"`
	Role             []string          `json:"role,omitempty" yaml:"role,omitempty"`
	SSHAgentAuth     bool              `json:"sshAgentAuth,omitempty" yaml:"sshAgentAuth,omitempty"`
	SSHCert          string            `json:"sshCert,omitempty" yaml:"sshCert,omitempty"`
	SSHCertPath      string            `json:"sshCertPath,omitempty" yaml:"sshCertPath,omitempty"`
	SSHKey           string            `json:"sshKey,omitempty" yaml:"sshKey,omitempty"`
	SSHKeyPath       string            `json:"sshKeyPath,omitempty" yaml:"sshKeyPath,omitempty"`
	Taints           []RKETaint        `json:"taints,omitempty" yaml:"taints,omitempty"`
	User             string            `json:"user,omitempty" yaml:"user,omitempty"`
}
