package client

const (
	QueryClusterGraphOutputType      = "queryClusterGraphOutput"
	QueryClusterGraphOutputFieldData = "data"
	QueryClusterGraphOutputFieldType = "type"
)

type QueryClusterGraphOutput struct {
	Data []QueryClusterGraph `json:"data,omitempty" yaml:"data,omitempty"`
	Type string              `json:"type,omitempty" yaml:"type,omitempty"`
}
