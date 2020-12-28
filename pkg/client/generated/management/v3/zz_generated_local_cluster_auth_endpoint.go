package client

const (
	LocalClusterAuthEndpointType         = "localClusterAuthEndpoint"
	LocalClusterAuthEndpointFieldCACerts = "caCerts"
	LocalClusterAuthEndpointFieldEnabled = "enabled"
	LocalClusterAuthEndpointFieldFQDN    = "fqdn"
)

type LocalClusterAuthEndpoint struct {
	CACerts string `json:"caCerts,omitempty" yaml:"caCerts,omitempty"`
	Enabled bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	FQDN    string `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`
}
