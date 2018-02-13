package client

const (
	ClusterRegistrationTokenStatusType             = "clusterRegistrationTokenStatus"
	ClusterRegistrationTokenStatusFieldCommand     = "command"
	ClusterRegistrationTokenStatusFieldManifestURL = "manifestUrl"
	ClusterRegistrationTokenStatusFieldNodeCommand = "nodeCommand"
	ClusterRegistrationTokenStatusFieldToken       = "token"
)

type ClusterRegistrationTokenStatus struct {
	Command     string `json:"command,omitempty"`
	ManifestURL string `json:"manifestUrl,omitempty"`
	NodeCommand string `json:"nodeCommand,omitempty"`
	Token       string `json:"token,omitempty"`
}
