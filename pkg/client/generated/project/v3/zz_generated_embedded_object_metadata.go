package client

const (
	EmbeddedObjectMetadataType             = "embeddedObjectMetadata"
	EmbeddedObjectMetadataFieldAnnotations = "annotations"
	EmbeddedObjectMetadataFieldLabels      = "labels"
	EmbeddedObjectMetadataFieldName        = "name"
)

type EmbeddedObjectMetadata struct {
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
}
