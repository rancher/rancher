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
	Command         string `json:"command,omitempty"`
	InsecureCommand string `json:"insecureCommand,omitempty"`
	ManifestURL     string `json:"manifestUrl,omitempty"`
	NodeCommand     string `json:"nodeCommand,omitempty"`
	Token           string `json:"token,omitempty"`
}
