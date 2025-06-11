package client

const (
	ClusterSecretsType                          = "clusterSecrets"
	ClusterSecretsFieldPrivateRegistryECRSecret = "privateRegistryECRSecret"
	ClusterSecretsFieldPrivateRegistrySecret    = "privateRegistrySecret"
	ClusterSecretsFieldPrivateRegistryURL       = "privateRegistryURL"
)

type ClusterSecrets struct {
	PrivateRegistryECRSecret string `json:"privateRegistryECRSecret,omitempty" yaml:"privateRegistryECRSecret,omitempty"`
	PrivateRegistrySecret    string `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	PrivateRegistryURL       string `json:"privateRegistryURL,omitempty" yaml:"privateRegistryURL,omitempty"`
}
