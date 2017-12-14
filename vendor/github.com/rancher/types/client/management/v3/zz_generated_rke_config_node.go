package client

const (
	RKEConfigNodeType                  = "rkeConfigNode"
	RKEConfigNodeFieldAddress          = "address"
	RKEConfigNodeFieldDockerSocket     = "dockerSocket"
	RKEConfigNodeFieldHostnameOverride = "hostnameOverride"
	RKEConfigNodeFieldInternalAddress  = "internalAddress"
	RKEConfigNodeFieldMachineName      = "machineName"
	RKEConfigNodeFieldRole             = "role"
	RKEConfigNodeFieldSSHKey           = "sshKey"
	RKEConfigNodeFieldSSHKeyPath       = "sshKeyPath"
	RKEConfigNodeFieldUser             = "user"
)

type RKEConfigNode struct {
	Address          string   `json:"address,omitempty"`
	DockerSocket     string   `json:"dockerSocket,omitempty"`
	HostnameOverride string   `json:"hostnameOverride,omitempty"`
	InternalAddress  string   `json:"internalAddress,omitempty"`
	MachineName      string   `json:"machineName,omitempty"`
	Role             []string `json:"role,omitempty"`
	SSHKey           string   `json:"sshKey,omitempty"`
	SSHKeyPath       string   `json:"sshKeyPath,omitempty"`
	User             string   `json:"user,omitempty"`
}
