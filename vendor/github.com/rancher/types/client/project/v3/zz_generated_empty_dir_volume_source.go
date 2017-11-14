package client

const (
	EmptyDirVolumeSourceType           = "emptyDirVolumeSource"
	EmptyDirVolumeSourceFieldMedium    = "medium"
	EmptyDirVolumeSourceFieldSizeLimit = "sizeLimit"
)

type EmptyDirVolumeSource struct {
	Medium    string `json:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty"`
}
