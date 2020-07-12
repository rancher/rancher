package client

const (
	ConfigMapNodeConfigSourceType                  = "configMapNodeConfigSource"
	ConfigMapNodeConfigSourceFieldKubeletConfigKey = "kubeletConfigKey"
	ConfigMapNodeConfigSourceFieldName             = "name"
	ConfigMapNodeConfigSourceFieldNamespace        = "namespace"
	ConfigMapNodeConfigSourceFieldResourceVersion  = "resourceVersion"
	ConfigMapNodeConfigSourceFieldUID              = "uid"
)

type ConfigMapNodeConfigSource struct {
	KubeletConfigKey string `json:"kubeletConfigKey,omitempty" yaml:"kubeletConfigKey,omitempty"`
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace        string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ResourceVersion  string `json:"resourceVersion,omitempty" yaml:"resourceVersion,omitempty"`
	UID              string `json:"uid,omitempty" yaml:"uid,omitempty"`
}
