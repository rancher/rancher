package client

const (
	ImageVolumeSourceType            = "imageVolumeSource"
	ImageVolumeSourceFieldPullPolicy = "pullPolicy"
	ImageVolumeSourceFieldReference  = "reference"
)

type ImageVolumeSource struct {
	PullPolicy string `json:"pullPolicy,omitempty" yaml:"pullPolicy,omitempty"`
	Reference  string `json:"reference,omitempty" yaml:"reference,omitempty"`
}
