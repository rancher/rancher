package client

const (
	ImageVolumeStatusType          = "imageVolumeStatus"
	ImageVolumeStatusFieldImageRef = "imageRef"
)

type ImageVolumeStatus struct {
	ImageRef string `json:"imageRef,omitempty" yaml:"imageRef,omitempty"`
}
