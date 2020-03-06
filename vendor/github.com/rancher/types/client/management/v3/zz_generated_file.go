package client

const (
	FileType          = "file"
	FileFieldContents = "contents"
	FileFieldName     = "name"
)

type File struct {
	Contents string `json:"contents,omitempty" yaml:"contents,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
}
