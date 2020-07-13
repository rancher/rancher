package client

const (
	QueryProjectGraphOutputType      = "queryProjectGraphOutput"
	QueryProjectGraphOutputFieldData = "data"
	QueryProjectGraphOutputFieldType = "type"
)

type QueryProjectGraphOutput struct {
	Data []QueryProjectGraph `json:"data,omitempty" yaml:"data,omitempty"`
	Type string              `json:"type,omitempty" yaml:"type,omitempty"`
}
