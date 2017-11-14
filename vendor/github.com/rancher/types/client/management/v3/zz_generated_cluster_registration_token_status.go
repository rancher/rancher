package client

const (
	ClusterRegistrationTokenStatusType             = "clusterRegistrationTokenStatus"
	ClusterRegistrationTokenStatusFieldCommand     = "command"
	ClusterRegistrationTokenStatusFieldManifestURL = "manifestUrl"
	ClusterRegistrationTokenStatusFieldToken       = "token"
)

type ClusterRegistrationTokenStatus struct {
	Command     string `json:"command,omitempty"`
	ManifestURL string `json:"manifestUrl,omitempty"`
	Token       string `json:"token,omitempty"`
}
