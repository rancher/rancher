package client

const (
	NodeConfigSourceType           = "nodeConfigSource"
	NodeConfigSourceFieldConfigMap = "configMap"
)

type NodeConfigSource struct {
	ConfigMap *ConfigMapNodeConfigSource `json:"configMap,omitempty" yaml:"configMap,omitempty"`
}
