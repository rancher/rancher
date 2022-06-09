package client

const (
	EmptyDirVolumeSourceType           = "emptyDirVolumeSource"
	EmptyDirVolumeSourceFieldMedium    = "medium"
	EmptyDirVolumeSourceFieldSizeLimit = "sizeLimit"
)

type EmptyDirVolumeSource struct {
	Medium    string `json:"medium,omitempty" yaml:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty" yaml:"sizeLimit,omitempty"`
}
