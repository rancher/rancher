package client

const (
	NodeConfigSourceType              = "nodeConfigSource"
	NodeConfigSourceFieldAPIVersion   = "apiVersion"
	NodeConfigSourceFieldConfigMapRef = "configMapRef"
	NodeConfigSourceFieldKind         = "kind"
)

type NodeConfigSource struct {
	APIVersion   string           `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	ConfigMapRef *ObjectReference `json:"configMapRef,omitempty" yaml:"configMapRef,omitempty"`
	Kind         string           `json:"kind,omitempty" yaml:"kind,omitempty"`
}
