package client

const (
	MetadataUpdateType             = "metadataUpdate"
	MetadataUpdateFieldAnnotations = "annotations"
	MetadataUpdateFieldLabels      = "labels"
)

type MetadataUpdate struct {
	Annotations *MapDelta `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Labels      *MapDelta `json:"labels,omitempty" yaml:"labels,omitempty"`
}
