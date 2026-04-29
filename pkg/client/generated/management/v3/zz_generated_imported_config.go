package client

const (
	ImportedConfigType                            = "importedConfig"
	ImportedConfigFieldKubeConfig                 = "kubeConfig"
	ImportedConfigFieldPrivateRegistryPullSecrets = "privateRegistryPullSecrets"
	ImportedConfigFieldPrivateRegistryURL         = "privateRegistryURL"
)

type ImportedConfig struct {
	KubeConfig                 string   `json:"kubeConfig,omitempty" yaml:"kubeConfig,omitempty"`
	PrivateRegistryPullSecrets []string `json:"privateRegistryPullSecrets,omitempty" yaml:"privateRegistryPullSecrets,omitempty"`
	PrivateRegistryURL         string   `json:"privateRegistryURL,omitempty" yaml:"privateRegistryURL,omitempty"`
}
