package client

const (
	ImportClusterYamlInputType                  = "importClusterYamlInput"
	ImportClusterYamlInputFieldDefaultNamespace = "defaultNamespace"
	ImportClusterYamlInputFieldNamespace        = "namespace"
	ImportClusterYamlInputFieldProjectID        = "projectId"
	ImportClusterYamlInputFieldYAML             = "yaml"
)

type ImportClusterYamlInput struct {
	DefaultNamespace string `json:"defaultNamespace,omitempty" yaml:"defaultNamespace,omitempty"`
	Namespace        string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ProjectID        string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	YAML             string `json:"yaml,omitempty" yaml:"yaml,omitempty"`
}
