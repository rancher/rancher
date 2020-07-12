package client

const (
	ContainerImageType           = "containerImage"
	ContainerImageFieldNames     = "names"
	ContainerImageFieldSizeBytes = "sizeBytes"
)

type ContainerImage struct {
	Names     []string `json:"names,omitempty" yaml:"names,omitempty"`
	SizeBytes int64    `json:"sizeBytes,omitempty" yaml:"sizeBytes,omitempty"`
}
