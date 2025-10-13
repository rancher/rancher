package client

const (
	FileKeySelectorType            = "fileKeySelector"
	FileKeySelectorFieldKey        = "key"
	FileKeySelectorFieldOptional   = "optional"
	FileKeySelectorFieldPath       = "path"
	FileKeySelectorFieldVolumeName = "volumeName"
)

type FileKeySelector struct {
	Key        string `json:"key,omitempty" yaml:"key,omitempty"`
	Optional   *bool  `json:"optional,omitempty" yaml:"optional,omitempty"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	VolumeName string `json:"volumeName,omitempty" yaml:"volumeName,omitempty"`
}
