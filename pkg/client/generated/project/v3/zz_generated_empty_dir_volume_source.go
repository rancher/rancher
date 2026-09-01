package client

const (
	EmptyDirVolumeSourceType           = "emptyDirVolumeSource"
	EmptyDirVolumeSourceFieldMedium    = "medium"
	EmptyDirVolumeSourceFieldMode      = "mode"
	EmptyDirVolumeSourceFieldSizeLimit = "sizeLimit"
)

type EmptyDirVolumeSource struct {
	Medium    string `json:"medium,omitempty" yaml:"medium,omitempty"`
	Mode      *int64 `json:"mode,omitempty" yaml:"mode,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty" yaml:"sizeLimit,omitempty"`
}
