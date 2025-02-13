package client

const (
	ImportedConfigType                    = "importedConfig"
	ImportedConfigFieldKubeConfig         = "kubeConfig"
	ImportedConfigFieldPrivateRegistryURL = "privateRegistryURL"
)

type ImportedConfig struct {
	KubeConfig         string `json:"kubeConfig,omitempty" yaml:"kubeConfig,omitempty"`
	PrivateRegistryURL string `json:"privateRegistryURL,omitempty" yaml:"privateRegistryURL,omitempty"`
}
