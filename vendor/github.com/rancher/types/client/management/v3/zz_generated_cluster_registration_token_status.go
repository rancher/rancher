package client

const (
	ClusterRegistrationTokenStatusType                 = "clusterRegistrationTokenStatus"
	ClusterRegistrationTokenStatusFieldCommand         = "command"
	ClusterRegistrationTokenStatusFieldInsecureCommand = "insecureCommand"
	ClusterRegistrationTokenStatusFieldManifestURL     = "manifestUrl"
	ClusterRegistrationTokenStatusFieldNodeCommand     = "nodeCommand"
	ClusterRegistrationTokenStatusFieldToken           = "token"
)

type ClusterRegistrationTokenStatus struct {
	Command         string `json:"command,omitempty" yaml:"command,omitempty"`
	InsecureCommand string `json:"insecureCommand,omitempty" yaml:"insecureCommand,omitempty"`
	ManifestURL     string `json:"manifestUrl,omitempty" yaml:"manifestUrl,omitempty"`
	NodeCommand     string `json:"nodeCommand,omitempty" yaml:"nodeCommand,omitempty"`
	Token           string `json:"token,omitempty" yaml:"token,omitempty"`
}
