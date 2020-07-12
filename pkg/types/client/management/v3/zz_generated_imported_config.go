package client

const (
	ImportedConfigType            = "importedConfig"
	ImportedConfigFieldKubeConfig = "kubeConfig"
)

type ImportedConfig struct {
	KubeConfig string `json:"kubeConfig,omitempty" yaml:"kubeConfig,omitempty"`
}
