package client

const (
	VolumeStatusType       = "volumeStatus"
	VolumeStatusFieldImage = "image"
)

type VolumeStatus struct {
	Image *ImageVolumeStatus `json:"image,omitempty" yaml:"image,omitempty"`
}
